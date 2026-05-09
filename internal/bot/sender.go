package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"time"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/vkgroup"
)

type Sender struct {
	vk            *vkgroup.Client
	msgRepo       *repository.MessageRepo
	userRepo      *repository.UserRepo
	broadcastRepo *repository.BroadcastRepo
	stateMgr      flows.StateMgr
	http          *http.Client
}

func NewSender(vk *vkgroup.Client, msgRepo *repository.MessageRepo, userRepo *repository.UserRepo, broadcastRepo *repository.BroadcastRepo, stateMgr flows.StateMgr) *Sender {
	return &Sender{
		vk:            vk,
		msgRepo:       msgRepo,
		userRepo:      userRepo,
		broadcastRepo: broadcastRepo,
		stateMgr:      stateMgr,
		http:          &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Sender) SendMsg(ctx context.Context, vkID int64, key string, kbJSON string) error {
	msg, err := s.msgRepo.Get(ctx, key)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("не удалось получить сообщение из БД")
		return err
	}
	if kbJSON == "" {
		kbJSON = flows.RenderContentKeyboard(msg.Keyboard, flows.KeyboardRenderOptions{})
	}
	return s.SendScreen(ctx, vkID, &flows.ScreenMessage{
		Key:      key,
		Text:     msg.Text,
		ImageURL: msg.ImageURL,
		Keyboard: kbJSON,
		CacheKey: key,
	})
}

func (s *Sender) SendText(ctx context.Context, vkID int64, text string, kbJSON string) error {
	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:   vkID,
		Text:     text,
		Keyboard: kbJSON,
		RandomID: uniqueID(),
	})
}

func (s *Sender) SendPhoto(ctx context.Context, vkID int64, photoURL, caption, kbJSON string) error {
	attachment, err := s.uploadPhotoFromURL(ctx, vkID, photoURL)
	if err != nil {
		log.Warn().Err(err).Msg("первая попытка загрузки фото в VK не удалась, повтор через 3с")
		time.Sleep(3 * time.Second)
		attachment, err = s.uploadPhotoFromURL(ctx, vkID, photoURL)
		if err != nil {
			return fmt.Errorf("ошибка загрузки фото в VK: %w", err)
		}
	}
	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       caption,
		Attachment: attachment,
		Keyboard:   kbJSON,
		RandomID:   uniqueID(),
	})
}

func (s *Sender) SendBroadcast(ctx context.Context, vkID int64, text string, imageURL *string, broadcastID int64) error {
	if imageURL == nil || *imageURL == "" {
		return s.SendText(ctx, vkID, text, "")
	}

	attachment, err := s.resolveBroadcastAttachment(ctx, vkID, *imageURL, broadcastID)
	if err != nil {
		return err
	}

	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       text,
		Attachment: attachment,
		RandomID:   uniqueID(),
	})
}

func (s *Sender) SendScreen(ctx context.Context, vkID int64, screen *flows.ScreenMessage) error {
	if screen == nil {
		return nil
	}

	var attachment string
	if screen.ImageURL != nil && *screen.ImageURL != "" {
		resolved, err := s.resolveAttachment(ctx, vkID, *screen.ImageURL, screen.CacheKey)
		if err != nil {
			if screen.CacheKey == "" {
				return err
			}
			log.Warn().Err(err).Str("screen_key", screen.Key).Msg("не удалось прикрепить изображение экрана")
		} else {
			attachment = resolved
		}
	}

	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       screen.Text,
		Attachment: attachment,
		Keyboard:   screen.Keyboard,
		RandomID:   uniqueID(),
	})
}

func (s *Sender) SendScreenText(ctx context.Context, vkID int64, key string, data map[string]any) error {
	return s.sendContentScreen(ctx, vkID, key, data, flows.KeyboardRenderOptions{}, nil)
}

func (s *Sender) SendTextToUser(ctx context.Context, vkID int64, text string) error {
	return s.SendText(ctx, vkID, text, "")
}

func (s *Sender) SendPhotoResult(ctx context.Context, vkID int64, photoURL, model, resolution, aspectRatio string) error {
	screenKey := "after_gen_free"
	kbOpts := flows.KeyboardRenderOptions{}

	resolutionLabel := resolution
	if resolutionLabel == "" {
		resolutionLabel = "1k"
	}
	aspectRatioLabel := aspectRatio
	if aspectRatioLabel == "" {
		aspectRatioLabel = "авто"
	}

	data := map[string]any{
		"ModelName":   flows.ModelDisplayName(model),
		"Resolution":  resolutionLabel,
		"AspectRatio": aspectRatioLabel,
	}

	if s.userRepo != nil {
		if u, err := s.userRepo.GetByVKID(ctx, vkID); err == nil && u != nil && (u.PaidGens > 0 || u.Status == "paid") {
			screenKey = "after_gen_paid"
			kbOpts.Links = map[string]string{"download_photo": photoURL}
		}
	}

	if err := s.sendContentScreen(ctx, vkID, screenKey, data, kbOpts, &photoURL); err != nil {
		return err
	}

	st, err := s.stateMgr.Get(ctx, vkID)
	if err != nil || st == nil {
		st = &flows.State{}
	}
	st.Step = flows.StepAfterGen
	st.PhotoURL = photoURL
	st.Model = model
	st.Resolution = resolution
	st.AspectRatio = aspectRatio
	_ = s.stateMgr.Set(ctx, vkID, st)
	return nil
}

func (s *Sender) sendContentScreen(ctx context.Context, vkID int64, key string, data map[string]any, kbOpts flows.KeyboardRenderOptions, imageOverride *string) error {
	msg, err := s.msgRepo.Get(ctx, key)
	if err != nil {
		return err
	}

	text, err := content.RenderText(msg.Text, data)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("не удалось отрендерить текст экрана")
	}

	imageURL := msg.ImageURL
	cacheKey := key
	if imageOverride != nil {
		imageURL = imageOverride
		cacheKey = ""
	}

	return s.SendScreen(ctx, vkID, &flows.ScreenMessage{
		Key:      key,
		Text:     text,
		ImageURL: imageURL,
		Keyboard: flows.RenderContentKeyboard(msg.Keyboard, kbOpts),
		CacheKey: cacheKey,
	})
}

func (s *Sender) resolveAttachment(ctx context.Context, vkID int64, imageURL, cacheKey string) (string, error) {
	if cacheKey != "" {
		msg, err := s.msgRepo.Get(ctx, cacheKey)
		if err == nil && msg != nil && msg.ImageURL != nil && *msg.ImageURL == imageURL && msg.VkAttachment != nil && *msg.VkAttachment != "" {
			return *msg.VkAttachment, nil
		}
	}

	attachment, err := s.uploadPhotoFromURL(ctx, vkID, imageURL)
	if err != nil {
		return "", err
	}

	if cacheKey != "" {
		if err := s.msgRepo.SetVkAttachment(ctx, cacheKey, attachment); err != nil {
			log.Warn().Err(err).Str("cache_key", cacheKey).Msg("не удалось сохранить vk_attachment в БД")
		}
	}
	return attachment, nil
}

func (s *Sender) resolveBroadcastAttachment(ctx context.Context, vkID int64, imageURL string, broadcastID int64) (string, error) {
	if broadcastID > 0 && s.broadcastRepo != nil {
		attachment, err := s.broadcastRepo.GetVKAttachment(ctx, broadcastID)
		if err == nil && attachment != nil && *attachment != "" {
			return *attachment, nil
		}
	}

	attachment, err := s.uploadPhotoFromURL(ctx, vkID, imageURL)
	if err != nil {
		return "", err
	}

	if broadcastID > 0 && s.broadcastRepo != nil {
		if err := s.broadcastRepo.SetVKAttachment(ctx, broadcastID, attachment); err != nil {
			log.Warn().Err(err).Int64("broadcast_id", broadcastID).Msg("не удалось сохранить vk_attachment рассылки")
		}
	}

	return attachment, nil
}

func (s *Sender) uploadPhotoFromURL(ctx context.Context, peerID int64, photoURL string) (string, error) {
	uploadURL, err := s.vk.GetPhotoUploadServer(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("getUploadServer: %w", err)
	}

	resp, err := s.http.Get(photoURL)
	if err != nil {
		return "", fmt.Errorf("скачивание фото: %w", err)
	}
	defer resp.Body.Close()
	photoData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("чтение тела фото: %w", err)
	}
	if len(photoData) == 0 {
		return "", fmt.Errorf("скачанный файл пустой: %s", photoURL)
	}

	filename := path.Base(photoURL)
	if filename == "." || filename == "/" || filename == "" {
		filename = "photo.png"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="photo"; filename="%s"`, filename))
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(photoData); err != nil {
		return "", err
	}
	_ = w.Close()

	uploadResp, err := s.http.Post(uploadURL, w.FormDataContentType(), &buf)
	if err != nil {
		return "", fmt.Errorf("загрузка на VK upload server: %w", err)
	}
	defer uploadResp.Body.Close()
	uploadBody, _ := io.ReadAll(uploadResp.Body)

	log.Info().
		Int("http_status", uploadResp.StatusCode).
		Str("body", string(uploadBody)).
		Msg("ответ VK upload server")

	if uploadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("VK upload server вернул HTTP %d: %s", uploadResp.StatusCode, string(uploadBody))
	}

	var uploadResult struct {
		Server int    `json:"server"`
		Photo  string `json:"photo"`
		Hash   string `json:"hash"`
	}
	if err := json.Unmarshal(uploadBody, &uploadResult); err != nil {
		return "", fmt.Errorf("парсинг ответа upload server: %w (тело: %s)", err, string(uploadBody))
	}
	if uploadResult.Photo == "" {
		return "", fmt.Errorf("VK upload server вернул пустое photo (HTTP %d): %s", uploadResp.StatusCode, string(uploadBody))
	}
	return s.vk.SaveMessagesPhoto(ctx, uploadResult.Server, uploadResult.Photo, uploadResult.Hash)
}

func uniqueID() int64 { return time.Now().UnixNano() }

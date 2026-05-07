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
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/vkgroup"
)

type Sender struct {
	vk       *vkgroup.Client
	msgRepo  *repository.MessageRepo
	userRepo *repository.UserRepo
	stateMgr flows.StateMgr
	http     *http.Client
}

func NewSender(vk *vkgroup.Client, msgRepo *repository.MessageRepo, userRepo *repository.UserRepo, stateMgr flows.StateMgr) *Sender {
	return &Sender{
		vk:       vk,
		msgRepo:  msgRepo,
		userRepo: userRepo,
		stateMgr: stateMgr,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// SendMsg реализует flows.Sender — отправляет сообщение по ключу из messages.
func (s *Sender) SendMsg(ctx context.Context, vkID int64, key string, kbJSON string) error {
	msg, err := s.msgRepo.Get(ctx, key)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("не удалось получить сообщение из БД")
		return err
	}

	var attachment string
	if msg.ImageURL != nil && *msg.ImageURL != "" {
		attach, uploadErr := s.uploadPhotoFromURL(ctx, vkID, *msg.ImageURL)
		if uploadErr != nil {
			log.Warn().Err(uploadErr).Msg("не удалось загрузить изображение в VK")
		} else {
			attachment = attach
		}
	}

	if kbJSON == "" && len(msg.Buttons) > 0 {
		kbJSON = flows.KbFromMsg(msg.Buttons)
	}

	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       msg.Text,
		Attachment: attachment,
		Keyboard:   kbJSON,
		RandomID:   uniqueID(),
	})
}

// SendText реализует flows.Sender — отправляет произвольный текст.
func (s *Sender) SendText(ctx context.Context, vkID int64, text string, kbJSON string) error {
	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:   vkID,
		Text:     text,
		Keyboard: kbJSON,
		RandomID: uniqueID(),
	})
}

// SendPhoto реализует flows.Sender — отправляет фото.
func (s *Sender) SendPhoto(ctx context.Context, vkID int64, photoURL, caption, kbJSON string) error {
	attachment, err := s.uploadPhotoFromURL(ctx, vkID, photoURL)
	if err != nil {
		return fmt.Errorf("ошибка загрузки фото в VK: %w", err)
	}
	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       caption,
		Attachment: attachment,
		Keyboard:   kbJSON,
		RandomID:   uniqueID(),
	})
}

// SendTextToUser реализует worker.MessageSender.
func (s *Sender) SendTextToUser(ctx context.Context, vkID int64, text string) error {
	return s.SendText(ctx, vkID, text, "")
}

// SendPhotoResult реализует worker.MessageSender — отправляет результат генерации с параметрами.
func (s *Sender) SendPhotoResult(ctx context.Context, vkID int64, photoURL, model, resolution, aspectRatio string) error {
	caption := "🎉 Готово! Вот твоя нейрофотосессия:"
	kb := flows.KbAfterGen()
	if s.userRepo != nil {
		if u, err := s.userRepo.GetByVKID(ctx, vkID); err == nil && u != nil && (u.PaidGens > 0 || u.Status == "paid") {
			res := resolution
			if res == "" {
				res = "1k"
			}
			ar := aspectRatio
			if ar == "" {
				ar = "авто"
			}
			caption = fmt.Sprintf("🎉 Готово!\n\n🤖 Модель: %s\n🔧 Качество: %s\n📐 Формат: %s",
				flows.ModelDisplayName(model), res, ar)
			kb = flows.KbAfterGenPaid(photoURL)
		}
	}
	if err := s.SendPhoto(ctx, vkID, photoURL, caption, kb); err != nil {
		return err
	}
	_ = s.stateMgr.Set(ctx, vkID, &flows.State{
		Step:        flows.StepAfterGen,
		PhotoURL:    photoURL,
		Model:       model,
		Resolution:  resolution,
		AspectRatio: aspectRatio,
	})
	return nil
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
		filename = "photo.jpg"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="photo"; filename="%s"`, filename))
	h.Set("Content-Type", "image/jpeg")
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

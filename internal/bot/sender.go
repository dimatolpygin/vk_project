package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	neturl "net/url"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/vkgroup"
	"vk_neuro_bot/internal/wavespeed"
)

type Sender struct {
	vk            *vkgroup.Client
	msgRepo       *repository.MessageRepo
	userRepo      *repository.UserRepo
	broadcastRepo *repository.BroadcastRepo
	stateMgr      flows.StateMgr
	http          *http.Client
	videoHTTP     *http.Client
}

const (
	vkPhotoDownloadAttempts = 3
	vkPhotoUploadAttempts   = 4
	videoResultScreenKey    = "after_gen_video"
	// Видео на 10 секунд весит десятки мегабайт, и качается оно с чужого CDN —
	// общего 30-секундного клиента на это не хватает.
	videoTransferTimeout = 5 * time.Minute
)

type vkUploadPhotoPayload struct {
	data        []byte
	filename    string
	contentType string
}

func NewSender(vk *vkgroup.Client, msgRepo *repository.MessageRepo, userRepo *repository.UserRepo, broadcastRepo *repository.BroadcastRepo, stateMgr flows.StateMgr) *Sender {
	return &Sender{
		vk:            vk,
		msgRepo:       msgRepo,
		userRepo:      userRepo,
		broadcastRepo: broadcastRepo,
		stateMgr:      stateMgr,
		http:          &http.Client{Timeout: 30 * time.Second},
		videoHTTP:     &http.Client{Timeout: videoTransferTimeout},
	}
}

func (s *Sender) SendMsg(ctx context.Context, vkID int64, key string, kbJSON string) error {
	msg, err := s.msgRepo.Get(ctx, key)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to load message from db")
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
		return fmt.Errorf("upload photo to vk: %w", err)
	}
	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       caption,
		Attachment: attachment,
		Keyboard:   kbJSON,
		RandomID:   uniqueID(),
	})
}

func (s *Sender) SendBroadcast(ctx context.Context, vkID int64, text string, imageURL *string, broadcastID int64, ctaEnabled bool) error {
	var keyboard string
	if ctaEnabled {
		keyboard = flows.KbBroadcastCTA()
	}

	if imageURL == nil || *imageURL == "" {
		return s.SendText(ctx, vkID, text, keyboard)
	}

	attachment, err := s.resolveBroadcastAttachment(ctx, vkID, *imageURL, broadcastID)
	if err != nil {
		return err
	}

	return s.vk.SendMessage(ctx, vkgroup.SendMessageParams{
		PeerID:     vkID,
		Text:       text,
		Attachment: attachment,
		Keyboard:   keyboard,
		RandomID:   uniqueID(),
	})
}

// SendPaymentReminder отправляет догоняющее сообщение с кнопкой покупки конкретного тарифа.
func (s *Sender) SendPaymentReminder(ctx context.Context, vkID int64, tariff *repository.Tariff) error {
	if tariff == nil {
		return fmt.Errorf("payment reminder: tariff is nil")
	}

	const key = "payment_reminder"
	msg, err := s.msgRepo.Get(ctx, key)
	if err != nil {
		return err
	}

	text, err := content.RenderText(msg.Text, map[string]any{
		"TariffName": tariff.Name,
		"Price":      fmt.Sprintf("%.0f₽", tariff.Price),
		"GensCount":  tariff.GensCount,
	})
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("failed to render payment reminder text")
	}

	return s.SendScreen(ctx, vkID, &flows.ScreenMessage{
		Key:      key,
		Text:     text,
		ImageURL: msg.ImageURL,
		Keyboard: flows.RenderContentKeyboardWithRows(
			msg.Keyboard,
			flows.TariffRows([]*repository.Tariff{tariff}),
			flows.KeyboardRenderOptions{},
		),
		CacheKey: key,
	})
}

func (s *Sender) SendScreen(ctx context.Context, vkID int64, screen *flows.ScreenMessage) error {
	if screen == nil {
		return nil
	}

	var attachment string
	if len(screen.Attachments) > 0 {
		attachment = strings.Join(screen.Attachments, ",")
	} else if screen.ImageURL != nil && *screen.ImageURL != "" {
		resolved, err := s.resolveAttachment(ctx, vkID, *screen.ImageURL, screen.CacheKey)
		if err != nil {
			if screen.CacheKey == "" {
				return err
			}
			log.Warn().Err(err).Str("screen_key", screen.Key).Msg("failed to attach screen image")
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
	return s.sendPhotoResult(ctx, vkID, photoURL, photoURL, model, resolution, aspectRatio)
}

func (s *Sender) SendPhotoResultWithSource(ctx context.Context, vkID int64, photoURL, sourcePhotoURL, model, resolution, aspectRatio string) error {
	if sourcePhotoURL == "" {
		sourcePhotoURL = photoURL
	}
	return s.sendPhotoResult(ctx, vkID, photoURL, sourcePhotoURL, model, resolution, aspectRatio)
}

func (s *Sender) sendPhotoResult(ctx context.Context, vkID int64, photoURL, sourcePhotoURL, model, resolution, aspectRatio string) error {
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

	if err := s.sendContentScreen(ctx, vkID, screenKey, data, kbOpts, &sourcePhotoURL); err != nil {
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

// SendVideoResult отдаёт готовое видео пользователю.
//
// Видео уходит документом: групповому токену метод video.save недоступен
// («invalid token type»), а mp4-документ ВК показывает со встроенным плеером.
// Если загрузка не удалась, экран всё равно отправляется — со ссылкой на файл
// в кнопке: терять оплаченное видео из-за отказа загрузчика нельзя.
func (s *Sender) SendVideoResult(ctx context.Context, vkID int64, videoURL string) error {
	data := map[string]any{"Duration": wavespeed.VideoDuration}
	kbOpts := flows.KeyboardRenderOptions{Links: map[string]string{"download_video": videoURL}}

	attachment, err := s.uploadVideoFromURL(ctx, vkID, videoURL)
	if err != nil {
		log.Error().Err(err).Int64("peer_id", vkID).Str("video_url", videoURL).
			Msg("не удалось загрузить видео в ВК, отдаём ссылкой")
	}

	msg, err := s.msgRepo.Get(ctx, videoResultScreenKey)
	if err != nil {
		return err
	}
	text, err := content.RenderText(msg.Text, data)
	if err != nil {
		log.Warn().Err(err).Str("key", videoResultScreenKey).Msg("failed to render video result text")
	}

	screen := &flows.ScreenMessage{
		Key:      videoResultScreenKey,
		Text:     text,
		Keyboard: flows.RenderContentKeyboard(msg.Keyboard, kbOpts),
	}
	if attachment != "" {
		screen.Attachments = []string{attachment}
	}
	if err := s.SendScreen(ctx, vkID, screen); err != nil {
		return err
	}

	st, err := s.stateMgr.Get(ctx, vkID)
	if err != nil || st == nil {
		st = &flows.State{}
	}
	st.Step = flows.StepMainMenu
	_ = s.stateMgr.Set(ctx, vkID, st)
	return nil
}

func (s *Sender) uploadVideoFromURL(ctx context.Context, peerID int64, videoURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.videoHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("download video: %w", err)
	}
	defer resp.Body.Close()

	videoData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read video body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("video source returned HTTP %d: %s", resp.StatusCode, abbreviateBody(videoData))
	}
	if len(videoData) == 0 {
		return "", fmt.Errorf("downloaded video is empty: %s", videoURL)
	}

	uploadURL, err := s.vk.GetDocUploadServer(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("docs.getMessagesUploadServer: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="video.mp4"`)
	h.Set("Content-Type", "video/mp4")
	fw, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(videoData); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", err
	}
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())

	uploadResp, err := s.videoHTTP.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("upload video to vk: %w", err)
	}
	defer uploadResp.Body.Close()
	uploadBody, _ := io.ReadAll(uploadResp.Body)

	if uploadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vk doc upload server returned HTTP %d: %s", uploadResp.StatusCode, abbreviateBody(uploadBody))
	}

	var uploadResult struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(uploadBody, &uploadResult); err != nil {
		return "", fmt.Errorf("parse doc upload response: %w (body: %s)", err, abbreviateBody(uploadBody))
	}
	if uploadResult.File == "" {
		return "", fmt.Errorf("vk doc upload server returned empty file: %s", abbreviateBody(uploadBody))
	}

	return s.vk.SaveDoc(ctx, uploadResult.File, "Нейровидео")
}

func (s *Sender) sendContentScreen(ctx context.Context, vkID int64, key string, data map[string]any, kbOpts flows.KeyboardRenderOptions, imageOverride *string) error {
	msg, err := s.msgRepo.Get(ctx, key)
	if err != nil {
		return err
	}

	text, err := content.RenderText(msg.Text, data)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("failed to render screen text")
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
			log.Warn().Err(err).Str("cache_key", cacheKey).Msg("failed to save vk_attachment")
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
			log.Warn().Err(err).Int64("broadcast_id", broadcastID).Msg("failed to save broadcast vk_attachment")
		}
	}

	return attachment, nil
}

func (s *Sender) uploadPhotoFromURL(ctx context.Context, peerID int64, photoURL string) (string, error) {
	payload, err := s.downloadPhotoForVK(ctx, photoURL)
	if err != nil {
		return "", err
	}
	return s.uploadPhotoToVK(ctx, peerID, photoURL, payload)
}

func (s *Sender) downloadPhotoForVK(ctx context.Context, photoURL string) (*vkUploadPhotoPayload, error) {
	var lastErr error
	for attempt := 1; attempt <= vkPhotoDownloadAttempts; attempt++ {
		payload, err := s.downloadPhotoForVKOnce(ctx, photoURL)
		if err == nil {
			return payload, nil
		}
		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Str("photo_url", photoURL).
			Msg("failed to download photo for vk delivery")
		if attempt < vkPhotoDownloadAttempts {
			if err := sleepWithContext(ctx, retryDelay(attempt)); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("download photo for vk: %w", lastErr)
}

func (s *Sender) downloadPhotoForVKOnce(ctx context.Context, photoURL string) (*vkUploadPhotoPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download photo: %w", err)
	}
	defer resp.Body.Close()

	photoData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read photo body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photo source returned HTTP %d, content_type=%q, body=%q",
			resp.StatusCode, resp.Header.Get("Content-Type"), abbreviateBody(photoData))
	}
	if len(photoData) == 0 {
		return nil, fmt.Errorf("downloaded photo is empty: %s", photoURL)
	}

	contentType, filename, err := normalizePhotoForVK(photoURL, resp.Header.Get("Content-Type"), photoData)
	if err != nil {
		return nil, err
	}

	return &vkUploadPhotoPayload{
		data:        photoData,
		filename:    filename,
		contentType: contentType,
	}, nil
}

func (s *Sender) uploadPhotoToVK(ctx context.Context, peerID int64, photoURL string, payload *vkUploadPhotoPayload) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= vkPhotoUploadAttempts; attempt++ {
		attachment, err := s.uploadPhotoToVKOnce(ctx, peerID, payload)
		if err == nil {
			if attempt > 1 {
				log.Info().
					Int("attempt", attempt).
					Int64("peer_id", peerID).
					Str("photo_url", photoURL).
					Int("bytes", len(payload.data)).
					Str("content_type", payload.contentType).
					Msg("photo delivered to vk after retry")
			}
			return attachment, nil
		}

		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int64("peer_id", peerID).
			Str("photo_url", photoURL).
			Int("bytes", len(payload.data)).
			Str("content_type", payload.contentType).
			Msg("failed to upload photo to vk")

		if attempt < vkPhotoUploadAttempts {
			if err := sleepWithContext(ctx, retryDelay(attempt)); err != nil {
				return "", err
			}
		}
	}
	return "", fmt.Errorf("vk upload failed after %d attempts: %w", vkPhotoUploadAttempts, lastErr)
}

func (s *Sender) uploadPhotoToVKOnce(ctx context.Context, peerID int64, payload *vkUploadPhotoPayload) (string, error) {
	uploadURL, err := s.vk.GetPhotoUploadServer(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("getUploadServer: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="photo"; filename="%s"`, payload.filename))
	h.Set("Content-Type", payload.contentType)
	fw, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(payload.data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	uploadResp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload to vk upload server: %w", err)
	}
	defer uploadResp.Body.Close()
	uploadBody, _ := io.ReadAll(uploadResp.Body)

	log.Info().
		Int("http_status", uploadResp.StatusCode).
		Str("body", string(uploadBody)).
		Msg("vk upload server response")

	if uploadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vk upload server returned HTTP %d: %s", uploadResp.StatusCode, string(uploadBody))
	}

	var uploadResult struct {
		Server int    `json:"server"`
		Photo  string `json:"photo"`
		Hash   string `json:"hash"`
	}
	if err := json.Unmarshal(uploadBody, &uploadResult); err != nil {
		return "", fmt.Errorf("parse upload server response: %w (body: %s)", err, string(uploadBody))
	}
	if uploadResult.Photo == "" {
		return "", fmt.Errorf("vk upload server returned empty photo (HTTP %d): %s", uploadResp.StatusCode, string(uploadBody))
	}
	return s.vk.SaveMessagesPhoto(ctx, uploadResult.Server, uploadResult.Photo, uploadResult.Hash)
}

func normalizePhotoForVK(photoURL, headerContentType string, photoData []byte) (string, string, error) {
	declaredContentType := normalizeContentType(headerContentType)
	detectedContentType := http.DetectContentType(photoData[:minInt(len(photoData), 512)])

	contentType := detectedContentType
	if !strings.HasPrefix(contentType, "image/") {
		if detectedContentType == "application/octet-stream" && strings.HasPrefix(declaredContentType, "image/") {
			contentType = declaredContentType
		} else {
			return "", "", fmt.Errorf("photo source did not return an image: header_content_type=%q detected_content_type=%q body=%q",
				headerContentType, detectedContentType, abbreviateBody(photoData))
		}
	}

	return contentType, buildVKUploadFilename(photoURL, contentType), nil
}

func normalizeContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(strings.Split(contentType, ";")[0]))
}

func buildVKUploadFilename(photoURL, contentType string) string {
	filename := path.Base(photoURL)
	if parsedURL, err := neturl.Parse(photoURL); err == nil {
		filename = path.Base(parsedURL.Path)
	}
	if filename == "." || filename == "/" || filename == "" {
		filename = "photo"
	}

	ext := preferredImageExtension(contentType)
	if ext == "" {
		ext = path.Ext(filename)
	}
	if ext == "" {
		ext = ".img"
	}

	base := strings.TrimSuffix(filename, path.Ext(filename))
	if base == "" {
		base = "photo"
	}
	return base + ext
}

func preferredImageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}

	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	for _, ext := range exts {
		if ext == ".jpe" {
			continue
		}
		return ext
	}
	return exts[0]
}

func abbreviateBody(body []byte) string {
	const limit = 160
	if len(body) == 0 {
		return ""
	}

	text := strings.ReplaceAll(string(body), "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Second
	case 2:
		return 2 * time.Second
	default:
		return 4 * time.Second
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func uniqueID() int64 { return time.Now().UnixNano() }

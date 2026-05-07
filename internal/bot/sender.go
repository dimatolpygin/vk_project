package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/vkgroup"
)

type Sender struct {
	vk       *vkgroup.Client
	msgRepo  *repository.MessageRepo
	stateMgr flows.StateMgr
	http     *http.Client
}

func NewSender(vk *vkgroup.Client, msgRepo *repository.MessageRepo, stateMgr flows.StateMgr) *Sender {
	return &Sender{
		vk:       vk,
		msgRepo:  msgRepo,
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

// SendPhotoToUser реализует worker.MessageSender.
func (s *Sender) SendPhotoToUser(ctx context.Context, vkID int64, photoURL string) error {
	if err := s.SendPhoto(ctx, vkID, photoURL, "🎉 Готово! Вот твоя нейрофотосессия:", flows.KbAfterGen()); err != nil {
		return err
	}
	_ = s.stateMgr.Set(ctx, vkID, &flows.State{Step: flows.StepAfterGen, PhotoURL: photoURL})
	return nil
}

func (s *Sender) uploadPhotoFromURL(ctx context.Context, peerID int64, photoURL string) (string, error) {
	uploadURL, err := s.vk.GetPhotoUploadServer(ctx, peerID)
	if err != nil {
		return "", err
	}

	resp, err := s.http.Get(photoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	photoData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("photo", filepath.Base(photoURL))
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(photoData); err != nil {
		return "", err
	}
	_ = w.Close()

	uploadResp, err := s.http.Post(uploadURL, w.FormDataContentType(), &buf)
	if err != nil {
		return "", err
	}
	defer uploadResp.Body.Close()
	uploadBody, _ := io.ReadAll(uploadResp.Body)

	var uploadResult struct {
		Server int    `json:"server"`
		Photo  string `json:"photo"`
		Hash   string `json:"hash"`
	}
	if err := json.Unmarshal(uploadBody, &uploadResult); err != nil {
		return "", err
	}
	return s.vk.SaveMessagesPhoto(ctx, uploadResult.Server, uploadResult.Photo, uploadResult.Hash)
}

func uniqueID() int64 { return time.Now().UnixNano() }

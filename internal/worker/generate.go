package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/wavespeed"
)

// MessageSender — минимальный интерфейс для отправки результата пользователю.
// Реализуется bot.Sender в пакете bot, инжектируется через main.go.
type MessageSender interface {
	SendTextToUser(ctx context.Context, vkID int64, text string) error
	SendPhotoResult(ctx context.Context, vkID int64, photoURL, model, resolution, aspectRatio string) error
}

// PhotoStorage — интерфейс S3-хранилища для загрузки выходных фото.
type PhotoStorage interface {
	UploadFromURL(ctx context.Context, key, url string) (string, error)
	PublicURL(key string) string
}

type GenerateHandler struct {
	genRepo *repository.GenerationRepo
	sender  MessageSender
	ws      *wavespeed.Client
	storage PhotoStorage
}

func NewGenerateHandler(
	genRepo *repository.GenerationRepo,
	sender MessageSender,
	ws *wavespeed.Client,
	storage PhotoStorage,
) *GenerateHandler {
	return &GenerateHandler{genRepo: genRepo, sender: sender, ws: ws, storage: storage}
}

func (h *GenerateHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	p, err := ParseGeneratePayload(t.Payload())
	if err != nil {
		return fmt.Errorf("генерация: ошибка парсинга payload: %w", err)
	}

	log.Info().
		Int64("generation_id", p.GenerationID).
		Int64("user_vk_id", p.UserVKID).
		Str("model", p.Model).
		Msg("начинаю генерацию")

	taskID, err := h.ws.Submit(ctx, wavespeed.SubmitRequest{
		Images:       p.Images,
		Prompt:       p.Prompt,
		Model:        p.Model,
		Resolution:   p.Resolution,
		AspectRatio:  p.AspectRatio,
		OutputFormat: p.OutputFormat,
	})
	if err != nil {
		_ = h.genRepo.SetFailed(ctx, p.GenerationID, err.Error())
		_ = h.sender.SendTextToUser(ctx, p.UserVKID, "❌ Ошибка при запуске генерации. Попробуй ещё раз.")
		return fmt.Errorf("%w: wavespeed submit: %v", asynq.SkipRetry, err)
	}

	if err := h.genRepo.SetWavespeedTaskID(ctx, p.GenerationID, taskID); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить task_id")
	}

	log.Info().Str("task_id", taskID).Msg("задача отправлена в WaveSpeed, жду результат")

	status, err := h.ws.PollUntilDone(ctx, taskID, 3*time.Second, 100)
	if err != nil {
		_ = h.genRepo.SetFailed(ctx, p.GenerationID, err.Error())
		_ = h.sender.SendTextToUser(ctx, p.UserVKID, "❌ Генерация завершилась с ошибкой. Попробуй позже.")
		return fmt.Errorf("%w: wavespeed poll: %v", asynq.SkipRetry, err)
	}

	if len(status.Outputs) == 0 {
		_ = h.genRepo.SetFailed(ctx, p.GenerationID, "нет выходных данных")
		_ = h.sender.SendTextToUser(ctx, p.UserVKID, "❌ Не удалось получить результат.")
		return fmt.Errorf("%w: wavespeed: нет выходных данных", asynq.SkipRetry)
	}

	outputURL := status.Outputs[0]

	if h.storage != nil {
		key := fmt.Sprintf("generation_users/%d/%d.png", p.UserVKID, p.GenerationID)
		if _, err := h.storage.UploadFromURL(ctx, key, outputURL); err != nil {
			log.Error().Err(err).Msg("не удалось загрузить результат в S3")
		} else {
			outputURL = h.storage.PublicURL(key)
		}
	}

	if err := h.genRepo.SetCompleted(ctx, p.GenerationID, outputURL); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить output_photo_url")
	}

	log.Info().
		Int64("generation_id", p.GenerationID).
		Str("output_url", outputURL).
		Msg("генерация завершена, отправляю фото")

	if err := h.sender.SendPhotoResult(ctx, p.UserVKID, outputURL, p.Model, p.Resolution, p.AspectRatio); err != nil {
		_ = h.sender.SendTextToUser(ctx, p.UserVKID, "❌ Фото создано, но не удалось отправить. Попробуй ещё раз.")
		return fmt.Errorf("%w: отправка фото: %v", asynq.SkipRetry, err)
	}

	return nil
}

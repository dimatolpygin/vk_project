package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
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
	genRepo  *repository.GenerationRepo
	userRepo *repository.UserRepo
	sender   MessageSender
	ws       *wavespeed.Client
	storage  PhotoStorage
	rdb      *redis.Client
}

func NewGenerateHandler(
	genRepo *repository.GenerationRepo,
	userRepo *repository.UserRepo,
	sender MessageSender,
	ws *wavespeed.Client,
	storage PhotoStorage,
	rdb *redis.Client,
) *GenerateHandler {
	return &GenerateHandler{genRepo: genRepo, userRepo: userRepo, sender: sender, ws: ws, storage: storage, rdb: rdb}
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

	// Сохраняем URL результата в Redis на случай сбоя при загрузке в VK
	if h.rdb != nil {
		key := fmt.Sprintf("gen_result:%d", p.GenerationID)
		_ = h.rdb.Set(ctx, key, outputURL, 24*time.Hour).Err()
	}

	if err := h.sender.SendPhotoResult(ctx, p.UserVKID, outputURL, p.Model, p.Resolution, p.AspectRatio); err != nil {
		log.Error().Err(err).Int64("generation_id", p.GenerationID).Msg("не удалось отправить фото после 2 попыток, возвращаю генерацию")
		if h.userRepo != nil {
			_ = h.userRepo.RefundGen(ctx, p.UserVKID)
		}
		_ = h.sender.SendTextToUser(ctx, p.UserVKID, "❌ Фото сгенерировано, но не удалось загрузить в VK. Генерация возвращена на твой баланс.")
		return fmt.Errorf("%w: отправка фото: %v", asynq.SkipRetry, err)
	}

	return nil
}

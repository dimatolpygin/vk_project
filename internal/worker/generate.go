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

type MessageSender interface {
	SendScreenText(ctx context.Context, vkID int64, key string, data map[string]any) error
	SendPhotoResult(ctx context.Context, vkID int64, photoURL, model, resolution, aspectRatio string) error
}

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
	return &GenerateHandler{
		genRepo:  genRepo,
		userRepo: userRepo,
		sender:   sender,
		ws:       ws,
		storage:  storage,
		rdb:      rdb,
	}
}

func (h *GenerateHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseGeneratePayload(task.Payload())
	if err != nil {
		return fmt.Errorf("генерация: ошибка парсинга payload: %w", err)
	}

	log.Info().
		Int64("generation_id", payload.GenerationID).
		Int64("user_vk_id", payload.UserVKID).
		Str("model", payload.Model).
		Msg("начинаю генерацию")

	taskID, err := h.ws.Submit(ctx, wavespeed.SubmitRequest{
		Images:       payload.Images,
		Prompt:       payload.Prompt,
		Model:        payload.Model,
		Resolution:   payload.Resolution,
		AspectRatio:  payload.AspectRatio,
		OutputFormat: payload.OutputFormat,
	})
	if err != nil {
		_ = h.genRepo.SetFailed(ctx, payload.GenerationID, err.Error())
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_submit_error", nil)
		return fmt.Errorf("%w: wavespeed submit: %v", asynq.SkipRetry, err)
	}

	if err := h.genRepo.SetWavespeedTaskID(ctx, payload.GenerationID, taskID); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить task_id")
	}

	log.Info().Str("task_id", taskID).Msg("задача отправлена в WaveSpeed, жду результат")

	status, err := h.ws.PollUntilDone(ctx, taskID, 3*time.Second, 100)
	if err != nil {
		_ = h.genRepo.SetFailed(ctx, payload.GenerationID, err.Error())
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_generation_failed", nil)
		return fmt.Errorf("%w: wavespeed poll: %v", asynq.SkipRetry, err)
	}

	if len(status.Outputs) == 0 {
		_ = h.genRepo.SetFailed(ctx, payload.GenerationID, "нет выходных данных")
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_no_output", nil)
		return fmt.Errorf("%w: wavespeed: нет выходных данных", asynq.SkipRetry)
	}

	outputURL := status.Outputs[0]
	if h.storage != nil {
		key := fmt.Sprintf("generation_users/%d/%d.png", payload.UserVKID, payload.GenerationID)
		if _, err := h.storage.UploadFromURL(ctx, key, outputURL); err != nil {
			log.Error().Err(err).Msg("не удалось загрузить результат в S3")
		} else {
			outputURL = h.storage.PublicURL(key)
		}
	}

	if err := h.genRepo.SetCompleted(ctx, payload.GenerationID, outputURL); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить output_photo_url")
	}

	log.Info().
		Int64("generation_id", payload.GenerationID).
		Str("output_url", outputURL).
		Msg("генерация завершена, отправляю фото")

	if h.rdb != nil {
		key := fmt.Sprintf("gen_result:%d", payload.GenerationID)
		_ = h.rdb.Set(ctx, key, outputURL, 24*time.Hour).Err()
	}

	if err := h.sender.SendPhotoResult(ctx, payload.UserVKID, outputURL, payload.Model, payload.Resolution, payload.AspectRatio); err != nil {
		log.Error().Err(err).Int64("generation_id", payload.GenerationID).Msg("не удалось отправить фото после 2 попыток, возвращаю генерацию")
		if h.userRepo != nil {
			_ = h.userRepo.RefundGen(ctx, payload.UserVKID)
		}
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_vk_upload_error", nil)
		return fmt.Errorf("%w: отправка фото: %v", asynq.SkipRetry, err)
	}

	return nil
}

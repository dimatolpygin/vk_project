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

type photoResultSourceSender interface {
	SendPhotoResultWithSource(ctx context.Context, vkID int64, photoURL, sourcePhotoURL, model, resolution, aspectRatio string) error
}

type PhotoStorage interface {
	UploadFromURL(ctx context.Context, key, url string) (string, error)
	PublicURL(key string) string
}

type GenerateHandler struct {
	genRepo *repository.GenerationRepo
	sender  MessageSender
	ws      *wavespeed.Client
	storage PhotoStorage
	rdb     *redis.Client
}

func NewGenerateHandler(
	genRepo *repository.GenerationRepo,
	sender MessageSender,
	ws *wavespeed.Client,
	storage PhotoStorage,
	rdb *redis.Client,
) *GenerateHandler {
	return &GenerateHandler{
		genRepo: genRepo,
		sender:  sender,
		ws:      ws,
		storage: storage,
		rdb:     rdb,
	}
}

func (h *GenerateHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseGeneratePayload(task.Payload())
	if err != nil {
		return fmt.Errorf("generation: parse payload: %w", err)
	}

	refundGeneration := func() {
		if h.genRepo == nil {
			return
		}
		if err := h.genRepo.RefundGenerationCharge(ctx, payload.GenerationID); err != nil {
			log.Error().Err(err).Int64("generation_id", payload.GenerationID).Msg("failed to refund generation charge")
		}
	}

	log.Info().
		Int64("generation_id", payload.GenerationID).
		Int64("user_vk_id", payload.UserVKID).
		Str("model", payload.Model).
		Msg("starting generation")

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
		refundGeneration()
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_submit_error", nil)
		return fmt.Errorf("%w: wavespeed submit: %v", asynq.SkipRetry, err)
	}

	if err := h.genRepo.SetWavespeedTaskID(ctx, payload.GenerationID, taskID); err != nil {
		log.Error().Err(err).Msg("failed to save wavespeed task id")
	}

	log.Info().Str("task_id", taskID).Msg("generation submitted to wavespeed")

	status, err := h.ws.PollUntilDone(ctx, taskID, 3*time.Second, 100)
	if err != nil {
		_ = h.genRepo.SetFailed(ctx, payload.GenerationID, err.Error())
		refundGeneration()
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_generation_failed", nil)
		return fmt.Errorf("%w: wavespeed poll: %v", asynq.SkipRetry, err)
	}

	if len(status.Outputs) == 0 {
		_ = h.genRepo.SetFailed(ctx, payload.GenerationID, "нет выходных данных")
		refundGeneration()
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_no_output", nil)
		return fmt.Errorf("%w: wavespeed: no outputs", asynq.SkipRetry)
	}

	deliveryURL := status.Outputs[0]
	outputURL := deliveryURL
	if h.storage != nil {
		key := fmt.Sprintf("generation_users/%d/%d.png", payload.UserVKID, payload.GenerationID)
		if _, err := h.storage.UploadFromURL(ctx, key, deliveryURL); err != nil {
			log.Error().Err(err).Msg("failed to upload result to s3")
		} else {
			outputURL = h.storage.PublicURL(key)
		}
	}

	if err := h.genRepo.SetCompleted(ctx, payload.GenerationID, outputURL); err != nil {
		log.Error().Err(err).Msg("failed to save generation output url")
	}

	log.Info().
		Int64("generation_id", payload.GenerationID).
		Str("delivery_url", deliveryURL).
		Str("output_url", outputURL).
		Msg("generation completed, sending photo")

	if h.rdb != nil {
		key := fmt.Sprintf("gen_result:%d", payload.GenerationID)
		_ = h.rdb.Set(ctx, key, outputURL, 24*time.Hour).Err()
	}

	var sendErr error
	if sourceAwareSender, ok := h.sender.(photoResultSourceSender); ok {
		sendErr = sourceAwareSender.SendPhotoResultWithSource(ctx, payload.UserVKID, outputURL, deliveryURL, payload.Model, payload.Resolution, payload.AspectRatio)
	} else {
		sendErr = h.sender.SendPhotoResult(ctx, payload.UserVKID, outputURL, payload.Model, payload.Resolution, payload.AspectRatio)
	}
	if sendErr != nil {
		log.Error().Err(sendErr).Int64("generation_id", payload.GenerationID).Msg("failed to send generated photo to vk, refunding generation")
		refundGeneration()
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_vk_upload_error", nil)
		return fmt.Errorf("%w: send photo: %v", asynq.SkipRetry, sendErr)
	}

	return nil
}

package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

const (
	defaultBroadcastBatchSize = 50
	broadcastTaskTimeout      = 2 * time.Minute
	broadcastProcessingTTL    = 10 * time.Minute
)

type BroadcastStore interface {
	Get(ctx context.Context, id int64) (*repository.Broadcast, error)
	ClaimPendingDeliveries(ctx context.Context, broadcastID int64, batchSize int, staleBefore time.Time) ([]*repository.BroadcastDelivery, error)
	MarkDeliverySent(ctx context.Context, deliveryID int64) error
	MarkDeliveryFailed(ctx context.Context, deliveryID int64, errText string) error
	RefreshStatus(ctx context.Context, broadcastID int64) (*repository.Broadcast, error)
	HasPendingDeliveries(ctx context.Context, broadcastID int64, staleBefore time.Time) (bool, error)
}

type BroadcastSender interface {
	SendBroadcast(ctx context.Context, vkID int64, text string, imageURL *string, broadcastID int64, ctaEnabled bool) error
}

type BroadcastTaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type BroadcastHandler struct {
	repo          BroadcastStore
	sender        BroadcastSender
	asynqClient   BroadcastTaskEnqueuer
	batchSize     int
	processingTTL time.Duration
}

func NewBroadcastHandler(repo BroadcastStore, sender BroadcastSender, asynqClient BroadcastTaskEnqueuer) *BroadcastHandler {
	return &BroadcastHandler{
		repo:          repo,
		sender:        sender,
		asynqClient:   asynqClient,
		batchSize:     defaultBroadcastBatchSize,
		processingTTL: broadcastProcessingTTL,
	}
}

func (h *BroadcastHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseBroadcastPayload(task.Payload())
	if err != nil {
		return fmt.Errorf("рассылка: ошибка парсинга payload: %w", err)
	}
	if payload.BroadcastID <= 0 {
		return fmt.Errorf("%w: invalid broadcast_id", asynq.SkipRetry)
	}

	broadcast, err := h.repo.Get(ctx, payload.BroadcastID)
	if err != nil {
		return err
	}
	if broadcast == nil {
		return fmt.Errorf("%w: broadcast %d not found", asynq.SkipRetry, payload.BroadcastID)
	}
	if broadcast.Status == repository.BroadcastStatusCompleted || broadcast.Status == repository.BroadcastStatusCompletedWithErrors {
		return nil
	}

	batchSize := payload.BatchSize
	if batchSize <= 0 {
		batchSize = h.batchSize
	}
	staleBefore := time.Now().UTC().Add(-h.processingTTL)

	deliveries, err := h.repo.ClaimPendingDeliveries(ctx, payload.BroadcastID, batchSize, staleBefore)
	if err != nil {
		return err
	}

	if len(deliveries) == 0 {
		if _, err := h.repo.RefreshStatus(ctx, payload.BroadcastID); err != nil {
			return err
		}
		return nil
	}

	for _, delivery := range deliveries {
		sendErr := h.sender.SendBroadcast(ctx, delivery.UserVKID, broadcast.Text, broadcast.ImageURL, broadcast.ID, broadcast.CTAEnabled)
		if sendErr != nil {
			log.Warn().
				Err(sendErr).
				Int64("broadcast_id", broadcast.ID).
				Int64("delivery_id", delivery.ID).
				Int64("user_vk_id", delivery.UserVKID).
				Msg("не удалось отправить сообщение рассылки")
			if err := h.repo.MarkDeliveryFailed(ctx, delivery.ID, sendErr.Error()); err != nil {
				return err
			}
			continue
		}

		if err := h.repo.MarkDeliverySent(ctx, delivery.ID); err != nil {
			return err
		}
	}

	updatedBroadcast, err := h.repo.RefreshStatus(ctx, payload.BroadcastID)
	if err != nil {
		return err
	}
	if updatedBroadcast == nil {
		return nil
	}

	hasPending, err := h.repo.HasPendingDeliveries(ctx, payload.BroadcastID, time.Now().UTC().Add(-h.processingTTL))
	if err != nil {
		return err
	}
	if !hasPending {
		return nil
	}
	if h.asynqClient == nil {
		return fmt.Errorf("broadcast queue client is nil")
	}

	nextPayload := BroadcastPayload{
		BroadcastID: payload.BroadcastID,
		BatchSize:   batchSize,
	}
	payloadBytes, err := nextPayload.Bytes()
	if err != nil {
		return err
	}

	nextTask := asynq.NewTask(
		TaskBroadcastProcess,
		payloadBytes,
		asynq.MaxRetry(5),
		asynq.Timeout(broadcastTaskTimeout),
	)
	if _, err := h.asynqClient.Enqueue(nextTask); err != nil {
		return err
	}

	return nil
}

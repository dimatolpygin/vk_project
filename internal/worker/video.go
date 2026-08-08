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

// VideoMessageSender — то, что видео-обработчику нужно от отправителя.
// SendVideoResult отдаёт mp4 в диалог; ошибку он возвращает только если
// не сработал и запасной вариант со ссылкой.
type VideoMessageSender interface {
	SendScreenText(ctx context.Context, vkID int64, key string, data map[string]any) error
	SendVideoResult(ctx context.Context, vkID int64, videoURL string) error
}

// Опрос двух звеньев цепочки. Фото собирается за минуту-полторы, видео на
// 10 секунд — единицы минут, поэтому интервалы и потолки у них разные.
const (
	videoScenePollInterval = 3 * time.Second
	videoScenePollAttempts = 100
	videoPollInterval      = 10 * time.Second
	videoPollAttempts      = 90
)

type GenerateVideoHandler struct {
	genRepo *repository.GenerationRepo
	sender  VideoMessageSender
	ws      *wavespeed.Client
	storage PhotoStorage
	rdb     *redis.Client
}

func NewGenerateVideoHandler(
	genRepo *repository.GenerationRepo,
	sender VideoMessageSender,
	ws *wavespeed.Client,
	storage PhotoStorage,
	rdb *redis.Client,
) *GenerateVideoHandler {
	return &GenerateVideoHandler{
		genRepo: genRepo,
		sender:  sender,
		ws:      ws,
		storage: storage,
		rdb:     rdb,
	}
}

func (h *GenerateVideoHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseGenerateVideoPayload(task.Payload())
	if err != nil {
		return fmt.Errorf("video generation: parse payload: %w", err)
	}

	// Видео стоит десятки генераций, поэтому любой обрыв цепочки обязан их
	// вернуть: пользователь не должен платить за то, чего не получил.
	fail := func(screenKey, errText string) {
		if h.genRepo != nil {
			_ = h.genRepo.SetFailed(ctx, payload.GenerationID, errText)
			if err := h.genRepo.RefundGenerationCharge(ctx, payload.GenerationID); err != nil {
				log.Error().Err(err).Int64("generation_id", payload.GenerationID).Msg("не удалось вернуть генерации за видео")
			}
		}
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, screenKey, nil)
	}

	log.Info().
		Int64("generation_id", payload.GenerationID).
		Int64("user_vk_id", payload.UserVKID).
		Str("photo_model", payload.PhotoModel).
		Int("image_count", len(payload.Images)).
		Int("cost_gens", payload.CostGens).
		Msg("старт видео-генерации")

	// ─── Звено 1: сцена ──────────────────────────────────────────────────────
	sceneURL, err := h.renderScene(ctx, payload)
	if err != nil {
		log.Error().Err(err).Int64("generation_id", payload.GenerationID).Msg("сцена для видео не собралась")
		fail("video_scene_failed", err.Error())
		return fmt.Errorf("%w: video scene: %v", asynq.SkipRetry, err)
	}

	if h.genRepo != nil {
		if err := h.genRepo.SetSceneReady(ctx, payload.GenerationID, sceneURL); err != nil {
			log.Error().Err(err).Msg("не удалось сохранить кадр сцены")
		}
	}
	// Экран ожидания уже висит у пользователя с момента постановки задачи,
	// но между звеньями проходят минуты — говорим, что процесс идёт.
	_ = h.sender.SendScreenText(ctx, payload.UserVKID, "video_scene_ready", map[string]any{
		"PromptName": payload.PromptName,
	})

	// ─── Звено 2: видео ──────────────────────────────────────────────────────
	videoURL, err := h.renderVideo(ctx, payload, sceneURL)
	if err != nil {
		log.Error().Err(err).Int64("generation_id", payload.GenerationID).Msg("видео не сгенерировалось")
		fail("video_generation_failed", err.Error())
		return fmt.Errorf("%w: video render: %v", asynq.SkipRetry, err)
	}

	deliveryURL := videoURL
	if h.storage != nil {
		key := fmt.Sprintf("generation_videos/%d/%d.mp4", payload.UserVKID, payload.GenerationID)
		if _, err := h.storage.UploadFromURL(ctx, key, videoURL); err != nil {
			// Ссылки WaveSpeed живут на их CDN и однажды протухнут, но для
			// доставки прямо сейчас годятся — падать из-за хранилища не за что.
			log.Error().Err(err).Msg("не удалось сохранить видео в хранилище")
		} else {
			deliveryURL = h.storage.PublicURL(key)
		}
	}

	if h.genRepo != nil {
		if err := h.genRepo.SetVideoCompleted(ctx, payload.GenerationID, sceneURL, deliveryURL); err != nil {
			log.Error().Err(err).Msg("не удалось сохранить ссылку на видео")
		}
	}

	if h.rdb != nil {
		key := fmt.Sprintf("gen_result:%d", payload.GenerationID)
		_ = h.rdb.Set(ctx, key, deliveryURL, 24*time.Hour).Err()
	}

	log.Info().
		Int64("generation_id", payload.GenerationID).
		Str("scene_url", sceneURL).
		Str("video_url", deliveryURL).
		Msg("видео готово, отправляем пользователю")

	if err := h.sender.SendVideoResult(ctx, payload.UserVKID, deliveryURL); err != nil {
		log.Error().Err(err).Int64("generation_id", payload.GenerationID).Msg("не удалось доставить видео в ВК, возвращаем генерации")
		if h.genRepo != nil {
			if refundErr := h.genRepo.RefundGenerationCharge(ctx, payload.GenerationID); refundErr != nil {
				log.Error().Err(refundErr).Int64("generation_id", payload.GenerationID).Msg("не удалось вернуть генерации после сбоя доставки")
			}
		}
		_ = h.sender.SendScreenText(ctx, payload.UserVKID, "worker_vk_upload_error", nil)
		return fmt.Errorf("%w: send video: %v", asynq.SkipRetry, err)
	}

	return nil
}

// renderScene — первое звено: фото-модель собирает по промту шаблона сцену
// из фото пользователя. Результат — обычная картинка, она же вход видео-модели.
func (h *GenerateVideoHandler) renderScene(ctx context.Context, payload *GenerateVideoPayload) (string, error) {
	taskID, err := h.ws.Submit(ctx, wavespeed.SubmitRequest{
		Images:       payload.Images,
		Prompt:       payload.Prompt,
		Model:        payload.PhotoModel,
		Resolution:   payload.Resolution,
		AspectRatio:  wavespeed.VideoAspectRatio,
		OutputFormat: payload.OutputFormat,
	})
	if err != nil {
		return "", fmt.Errorf("submit сцены: %w", err)
	}

	if h.genRepo != nil {
		if err := h.genRepo.SetWavespeedTaskID(ctx, payload.GenerationID, taskID); err != nil {
			log.Error().Err(err).Msg("не удалось сохранить task_id сцены")
		}
	}

	status, err := h.ws.PollUntilDone(ctx, taskID, videoScenePollInterval, videoScenePollAttempts)
	if err != nil {
		return "", fmt.Errorf("ожидание сцены: %w", err)
	}
	if len(status.Outputs) == 0 {
		return "", fmt.Errorf("фото-модель не вернула кадр сцены")
	}
	return status.Outputs[0], nil
}

// renderVideo — второе звено: сцена уходит в Seedance. Параметры фиксированные,
// пользователю они не предлагаются, поэтому и здесь не настраиваются.
func (h *GenerateVideoHandler) renderVideo(ctx context.Context, payload *GenerateVideoPayload, sceneURL string) (string, error) {
	taskID, err := h.ws.SubmitVideo(ctx, wavespeed.SubmitVideoRequest{
		Image:         sceneURL,
		Prompt:        payload.VideoPrompt,
		AspectRatio:   wavespeed.VideoAspectRatio,
		Resolution:    wavespeed.VideoResolution,
		Duration:      wavespeed.VideoDuration,
		GenerateAudio: true,
	})
	if err != nil {
		return "", fmt.Errorf("submit видео: %w", err)
	}

	if h.genRepo != nil {
		if err := h.genRepo.SetWavespeedTaskID(ctx, payload.GenerationID, taskID); err != nil {
			log.Error().Err(err).Msg("не удалось сохранить task_id видео")
		}
	}

	status, err := h.ws.PollUntilDone(ctx, taskID, videoPollInterval, videoPollAttempts)
	if err != nil {
		return "", fmt.Errorf("ожидание видео: %w", err)
	}
	if len(status.Outputs) == 0 {
		return "", fmt.Errorf("видео-модель не вернула файл")
	}
	return status.Outputs[0], nil
}

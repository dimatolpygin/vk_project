package flows

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/worker"
)

// Видео живёт в общей очереди отдельным именем: цепочка из двух моделей
// занимает воркер минутами, и фото-задачи не должны стоять за ней в хвосте.
const videoQueueName = "video"

// videoTaskTimeout берётся с запасом к потолкам опроса в воркере: сцена до
// пяти минут плюс видео до пятнадцати.
const videoTaskTimeout = 25 * time.Minute

// hasGensFor — хватает ли пользователю баланса на промт такой цены.
// Балансы складываются: списание умеет разложиться по обоим.
func hasGensFor(u *User, cost int) bool {
	if cost < 1 {
		cost = 1
	}
	return u.TotalGens() >= cost
}

// notEnoughGensData — цифры для экрана отказа: сколько стоит, сколько есть
// и сколько не хватает. Считаем здесь, чтобы текст экрана правился из админки.
func notEnoughGensData(u *User, cost int) map[string]any {
	available := u.TotalGens()
	missing := cost - available
	if missing < 0 {
		missing = 0
	}
	return map[string]any{
		"CostGens":    cost,
		"UserGens":    available,
		"MissingGens": missing,
	}
}

// handleVideoPromptSelected — выбран видео-тренд. Отличий от фото два: цена
// проверяется до запроса фото, и генерация уходит в свою задачу.
func handleVideoPromptSelected(ctx context.Context, fc *Context, d *Deps, prompt *repository.Prompt, promptType string) {
	cost := prompt.Cost()
	if !hasGensFor(fc.User, cost) {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_for_video", ScreenOptions{Data: notEnoughGensData(fc.User, cost)})
		return
	}

	state := copyPrefs(&State{
		Step:         StepAwaitingPhoto,
		PromptType:   promptType,
		TemplateID:   prompt.ID,
		Section:      fc.State.Section,
		CategoryID:   fc.State.CategoryID,
		CategoryPage: fc.State.CategoryPage,
		PromptPage:   fc.State.PromptPage,
	}, fc.State)
	fc.State = state
	_ = d.State.Set(ctx, fc.VkID, state)

	if fc.User.UseSavedPhoto && len(fc.User.SavedPhotoURLs) > 0 {
		startVideoGeneration(ctx, fc, d, fc.User.SavedPhotoURLs, prompt)
		return
	}

	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

// startVideoGeneration списывает цену промта и ставит задачу в очередь.
func startVideoGeneration(ctx context.Context, fc *Context, d *Deps, photoURLs []string, prompt *repository.Prompt) {
	cost := prompt.Cost()
	model := currentModel(fc, d)

	inputPhotoURL := firstGenerationInputPhotoURL(photoURLs)
	gen, err := d.GenRepo.CreateChargedGeneration(ctx, fc.VkID, "video", prompt.Prompt, model, inputPhotoURL, cost)
	switch {
	case errors.Is(err, repository.ErrNoGenerationsAvailable):
		// Баланс проверялся при выборе тренда, но между выбором и фото могла
		// пройти чужая генерация — показываем те же цифры, а не общий отказ.
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_for_video", ScreenOptions{Data: notEnoughGensData(fc.User, cost)})
		return
	case err != nil:
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("не удалось создать запись видео-генерации")
		_ = sendScreen(ctx, d, fc.VkID, "generation_error", ScreenOptions{})
		return
	}

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventGenerationStarted, "video_gen", "video_generating_wait", map[string]any{
		"generation_id": gen.ID,
		"prompt_type":   "video",
		"prompt_id":     prompt.ID,
		"model":         model,
		"cost_gens":     cost,
		"input_photos":  len(photoURLs),
	})

	nextState := *fc.State
	nextState.Step = StepMainMenu
	nextState.InputPhotoURLs = nil
	nextState.CouplePhotoURLs = nil
	nextState.PhotoBatchID = ""
	_ = d.State.Set(ctx, fc.VkID, &nextState)

	_ = sendScreen(ctx, d, fc.VkID, "video_generating_wait", ScreenOptions{Data: map[string]any{
		"PromptName": prompt.Name,
		"CostGens":   cost,
	}})

	payload := worker.GenerateVideoPayload{
		GenerationID: gen.ID,
		UserVKID:     fc.VkID,
		PhotoModel:   model,
		Images:       clonePhotoURLs(normalizeGenerationInputPhotos(photoURLs)),
		Prompt:       prompt.Prompt,
		VideoPrompt:  prompt.VideoPrompt,
		Resolution:   currentResolution(fc),
		OutputFormat: "png",
		PromptName:   prompt.Name,
		CostGens:     cost,
	}
	payloadBytes, _ := payload.Bytes()
	// MaxRetry(0): цепочка платная, и повтор после неизвестной ошибки означал бы
	// вторую пару вызовов моделей за те же деньги. Все известные сбои воркер
	// обрабатывает сам — с возвратом генераций.
	task := asynq.NewTask(worker.TaskGenerateVideo, payloadBytes,
		asynq.Queue(videoQueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(videoTaskTimeout),
	)
	if _, err := d.AsynqClient.Enqueue(task); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("не удалось поставить видео-задачу в очередь")
		_ = d.GenRepo.SetFailed(ctx, gen.ID, err.Error())
		if refundErr := d.GenRepo.RefundGenerationCharge(ctx, gen.ID); refundErr != nil {
			log.Error().Err(refundErr).Int64("generation_id", gen.ID).Msg("не удалось вернуть генерации после ошибки очереди")
		}
		_ = sendScreen(ctx, d, fc.VkID, "generation_start_error", ScreenOptions{})
	}
}

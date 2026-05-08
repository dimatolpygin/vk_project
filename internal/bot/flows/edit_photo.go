package flows

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/worker"
)

func HandleEditPhotoStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhotoEdit, PromptType: "edit"}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
}

func HandleEditResultStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{
		Step:     StepAwaitingResultEdit,
		PhotoURL: fc.State.PhotoURL,
	}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
}

func HandleResultEditPrompt(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || fc.Message.Text == "" {
		_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
		return
	}

	photoURL := fc.State.PhotoURL
	if photoURL == "" {
		_ = sendScreen(ctx, d, fc.VkID, "edit_result_missing_photo", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	launchEditGeneration(ctx, fc, d, photoURL, fc.Message.Text)
}

func launchEditGeneration(ctx context.Context, fc *Context, d *Deps, photoURL, prompt string) {
	model := fc.State.Model
	if model == "" {
		model = fc.User.PrefModel
	}
	if model == "" {
		model = d.DefaultModel
	}

	gen, err := d.GenRepo.Create(ctx, fc.VkID, "edit", prompt, model, &photoURL)
	if err != nil {
		log.Error().Err(err).Msg("не удалось создать запись генерации")
		_ = sendScreen(ctx, d, fc.VkID, "generation_error", ScreenOptions{})
		return
	}

	resolution := fc.State.Resolution
	if resolution == "" {
		resolution = fc.User.PrefResolution
	}
	if resolution == "" {
		resolution = "1k"
	}
	aspectRatio := fc.State.AspectRatio
	if aspectRatio == "" {
		aspectRatio = fc.User.PrefAspectRatio
	}

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventGenerationStarted, "edit_result", "generating_wait", map[string]any{
		"generation_id": gen.ID,
		"prompt_type":   "edit",
		"model":         model,
		"resolution":    resolution,
		"aspect_ratio":  aspectRatio,
	})
	_ = d.UserRepo.DecrementGens(ctx, fc.VkID)
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = sendScreen(ctx, d, fc.VkID, "generating_wait", ScreenOptions{})

	payload := worker.GeneratePayload{
		GenerationID: gen.ID,
		UserVKID:     fc.VkID,
		Model:        model,
		Images:       []string{photoURL},
		Prompt:       prompt,
		Resolution:   resolution,
		AspectRatio:  aspectRatio,
		OutputFormat: "png",
	}
	payloadBytes, _ := payload.Bytes()
	task := asynq.NewTask(worker.TaskGenerate, payloadBytes,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
	)
	if _, err := d.AsynqClient.Enqueue(task); err != nil {
		log.Error().Err(err).Msg("не удалось поставить задачу в очередь")
		_ = sendScreen(ctx, d, fc.VkID, "generation_start_error", ScreenOptions{})
	}
}

package flows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/worker"
)

func HandleAfterGen(ctx context.Context, fc *Context, d *Deps) {
	if fc.State.PhotoURL != "" {
		_ = showAfterGenScreen(ctx, fc, d, fc.State.PhotoURL)
		return
	}
	if fc.User.HasGens() {
		HandleMainMenu(ctx, fc, d)
	} else {
		HandleWelcome(ctx, fc, d)
	}
}

func HandleFreeGenStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		HandleShowTariffs(ctx, fc, d)
		return
	}

	isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка проверки подписки")
		isMember = false
	}

	if !isMember {
		_ = sendScreen(ctx, d, fc.VkID, "subscribe_cta", ScreenOptions{
			Links: map[string]string{"subscribe_group": d.VKGroupURL},
		})
		_ = d.State.SetStep(ctx, fc.VkID, StepFreeGenStart)
		return
	}

	proceedToPhotoRequest(ctx, fc, d, "free")
}

func HandleCheckSubscription(ctx context.Context, fc *Context, d *Deps) {
	isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
	if err != nil || !isMember {
		_ = sendScreen(ctx, d, fc.VkID, "subscribe_cta", ScreenOptions{
			Links: map[string]string{"subscribe_group": d.VKGroupURL},
		})
		return
	}

	_ = d.UserRepo.SetSubscribed(ctx, fc.VkID, true)
	trackEvent(ctx, d, fc.VkID, repository.ActivityEventSubscriptionConfirmed, "check_sub", "subscribe_cta", nil)
	proceedToPhotoRequest(ctx, fc, d, "free")
}

func proceedToPhotoRequest(ctx context.Context, fc *Context, d *Deps, promptType string) {
	if fc.User.Gender == "unknown" {
		_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingGender, PromptType: promptType}, fc.State))
		_ = sendScreen(ctx, d, fc.VkID, "gender_select", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhoto, PromptType: promptType}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleGenAgain(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		HandleShowTariffs(ctx, fc, d)
		return
	}

	if fc.User.Status != "paid" && fc.User.PaidGens == 0 {
		isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
		if err != nil {
			isMember = false
		}
		if !isMember {
			_ = sendScreen(ctx, d, fc.VkID, "subscribe_cta", ScreenOptions{
				Links: map[string]string{"subscribe_group": d.VKGroupURL},
			})
			_ = d.State.SetStep(ctx, fc.VkID, StepFreeGenStart)
			return
		}
	}

	if fc.User.Gender == "unknown" {
		savedState := *fc.State
		savedState.Step = StepAwaitingGender
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = sendScreen(ctx, d, fc.VkID, "gender_select", ScreenOptions{})
		return
	}

	savedState := *fc.State
	savedState.PhotoURL = ""

	if fc.State.PromptType == "edit" {
		savedState.Step = StepAwaitingPhotoEdit
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
	} else {
		savedState.Step = StepAwaitingPhoto
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
	}
}

func HandleGenderSelect(ctx context.Context, fc *Context, d *Deps, gender string) {
	_ = d.UserRepo.SetGender(ctx, fc.VkID, gender)
	fc.User.Gender = gender

	promptType := fc.State.PromptType
	if promptType == "" {
		promptType = "free"
	}

	newState := *fc.State
	newState.PromptType = promptType
	if promptType == "edit" {
		newState.Step = StepAwaitingPhotoEdit
		_ = d.State.Set(ctx, fc.VkID, &newState)
		_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
		return
	}

	newState.Step = StepAwaitingPhoto
	_ = d.State.Set(ctx, fc.VkID, &newState)
	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleAwaitingPhoto(ctx context.Context, fc *Context, d *Deps) {
	var photos []string
	if fc.Message != nil {
		photos = fc.Message.Photos
	}

	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	photoURL := photos[0]
	uploadedURL := photoURL
	if d.Storage != nil {
		key := fmt.Sprintf("user_upload/%d/%d.png", fc.VkID, time.Now().Unix())
		if _, err := d.Storage.UploadFromURL(ctx, key, photoURL); err != nil {
			log.Error().Err(err).Msg("не удалось загрузить фото в S3, используем VK URL")
		} else {
			uploadedURL = d.Storage.PublicURL(key)
		}
	}

	promptType := fc.State.PromptType
	prompt := buildDefaultPrompt(fc.User.Gender, promptType)
	if fc.State.CustomPrompt != "" {
		prompt = fc.State.CustomPrompt
	}
	if fc.State.TemplateID > 0 {
		if templatePrompt, err := d.PromptRepo.GetByID(ctx, fc.State.TemplateID); err == nil && templatePrompt != nil {
			prompt = templatePrompt.Prompt
		}
	}

	startGeneration(ctx, fc, d, uploadedURL, prompt, promptType, "generating_wait", nil)
}

func startGeneration(ctx context.Context, fc *Context, d *Deps, photoURL, prompt, promptType, waitKey string, waitData map[string]any) {
	createAndEnqueueGeneration(ctx, fc, d, promptType, photoURL, prompt, waitKey, "free_gen", waitData)
}

func createAndEnqueueGeneration(ctx context.Context, fc *Context, d *Deps, generationType, photoURL, prompt, waitKey, activityActionKey string, waitData map[string]any) {
	model := fc.State.Model
	if model == "" {
		model = fc.User.PrefModel
	}
	if model == "" {
		model = d.DefaultModel
	}

	gen, err := d.GenRepo.CreateChargedGeneration(ctx, fc.VkID, generationType, prompt, model, &photoURL)
	switch {
	case errors.Is(err, repository.ErrNoGenerationsAvailable):
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	case err != nil:
		log.Error().Err(err).Msg("не удалось создать запись генерации")
		_ = sendScreen(ctx, d, fc.VkID, "generation_error", ScreenOptions{})
		return
	}

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventGenerationStarted, activityActionKey, waitKey, map[string]any{
		"generation_id": gen.ID,
		"prompt_type":   generationType,
		"model":         model,
		"resolution":    currentResolution(fc),
		"aspect_ratio":  currentAspectRatio(fc),
	})
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = sendScreen(ctx, d, fc.VkID, waitKey, ScreenOptions{Data: waitData})

	resolution := currentResolution(fc)
	aspectRatio := currentAspectRatio(fc)
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
		_ = d.GenRepo.SetFailed(ctx, gen.ID, err.Error())
		if refundErr := d.GenRepo.RefundGenerationCharge(ctx, gen.ID); refundErr != nil {
			log.Error().Err(refundErr).Int64("generation_id", gen.ID).Msg("не удалось вернуть генерацию после enqueue error")
		}
		_ = sendScreen(ctx, d, fc.VkID, "generation_start_error", ScreenOptions{})
	}
}

func HandleAwaitingPrompt(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || fc.Message.Text == "" {
		_ = sendScreen(ctx, d, fc.VkID, "custom_prompt_intro", ScreenOptions{})
		return
	}

	state := fc.State
	state.Step = StepAwaitingPhoto
	state.CustomPrompt = fc.Message.Text
	_ = d.State.Set(ctx, fc.VkID, state)
	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleAwaitingPhotoEdit(ctx context.Context, fc *Context, d *Deps) {
	var photos []string
	if fc.Message != nil {
		photos = fc.Message.Photos
	}
	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	photoURL := photos[0]
	uploadedURL := photoURL
	if d.Storage != nil {
		key := fmt.Sprintf("user_upload/%d/%d.png", fc.VkID, time.Now().Unix())
		if _, err := d.Storage.UploadFromURL(ctx, key, photoURL); err != nil {
			log.Error().Err(err).Msg("не удалось загрузить фото в S3, используем VK URL")
		} else {
			uploadedURL = d.Storage.PublicURL(key)
		}
	}

	state := fc.State
	state.Step = StepAwaitingEditPrompt
	state.PhotoURL = uploadedURL
	_ = d.State.Set(ctx, fc.VkID, state)

	if state.CustomPrompt != "" {
		fc.State = state
		launchEditGeneration(ctx, fc, d, uploadedURL, state.CustomPrompt)
		return
	}

	_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
}

func showAfterGenScreen(ctx context.Context, fc *Context, d *Deps, photoURL string) error {
	screenKey := "after_gen_free"
	options := ScreenOptions{ImageOverride: &photoURL}

	if fc.User.PaidGens > 0 || fc.User.Status == "paid" {
		screenKey = "after_gen_paid"
		options.Links = map[string]string{"download_photo": photoURL}
		options.Data = map[string]any{
			"ModelName":   ModelDisplayName(currentModel(fc, d)),
			"Resolution":  currentResolution(fc),
			"AspectRatio": currentAspectRatioLabel(fc),
		}
	}

	return sendScreen(ctx, d, fc.VkID, screenKey, options)
}

func currentModel(fc *Context, d *Deps) string {
	if fc.State.Model != "" {
		return fc.State.Model
	}
	if fc.User.PrefModel != "" {
		return fc.User.PrefModel
	}
	return d.DefaultModel
}

func currentResolution(fc *Context) string {
	if fc.State.Resolution != "" {
		return fc.State.Resolution
	}
	if fc.User.PrefResolution != "" {
		return fc.User.PrefResolution
	}
	return "1k"
}

func currentAspectRatio(fc *Context) string {
	if fc.State.AspectRatio != "" {
		return fc.State.AspectRatio
	}
	return fc.User.PrefAspectRatio
}

func currentAspectRatioLabel(fc *Context) string {
	ar := currentAspectRatio(fc)
	if ar == "" {
		return "авто"
	}
	return ar
}

func buildDefaultPrompt(gender, promptType string) string {
	switch promptType {
	case "couple_pair":
		return "romantic couple portrait, two people, professional photo, studio lighting, high quality"
	case "couple_family":
		return "family portrait, warm atmosphere, professional photo, studio lighting, high quality"
	case "couple":
		return "couple portrait, two people, professional photo, studio lighting, high quality"
	}

	genderLabel := "woman"
	if gender == "male" {
		genderLabel = "man"
	}
	return fmt.Sprintf("professional portrait photo of a %s, studio lighting, high quality, photorealistic", genderLabel)
}

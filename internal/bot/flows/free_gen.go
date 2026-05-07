package flows

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/worker"
)

func HandleAfterGen(ctx context.Context, fc *Context, d *Deps) {
	if fc.State.PhotoURL != "" {
		kb := afterGenKb(fc)
		_ = d.Sender.SendPhoto(ctx, fc.VkID, fc.State.PhotoURL, "🎉 Готово! Вот твоя нейрофотосессия:", kb)
		return
	}
	if fc.User.HasGens() {
		HandleMainMenu(ctx, fc, d)
	} else {
		HandleWelcome(ctx, fc, d)
	}
}

func afterGenKb(fc *Context) string {
	if fc.User.PaidGens > 0 || fc.User.Status == "paid" {
		return KbAfterGenPaid(fc.State.PhotoURL)
	}
	return KbAfterGen()
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
		_ = d.Sender.SendMsg(ctx, fc.VkID, "subscribe_cta", KbSubscribeCTA(d.VKGroupURL))
		_ = d.State.SetStep(ctx, fc.VkID, StepFreeGenStart)
		return
	}

	proceedToPhotoRequest(ctx, fc, d, "free")
}

func HandleCheckSubscription(ctx context.Context, fc *Context, d *Deps) {
	isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
	if err != nil || !isMember {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "subscribe_cta", KbSubscribeCTA(d.VKGroupURL))
		return
	}

	_ = d.UserRepo.SetSubscribed(ctx, fc.VkID, true)
	proceedToPhotoRequest(ctx, fc, d, "free")
}

func proceedToPhotoRequest(ctx context.Context, fc *Context, d *Deps, promptType string) {
	if fc.User.Gender == "unknown" {
		_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingGender, PromptType: promptType}, fc.State))
		_ = d.Sender.SendMsg(ctx, fc.VkID, "gender_select", KbGender())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhoto, PromptType: promptType}, fc.State))
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
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
			_ = d.Sender.SendMsg(ctx, fc.VkID, "subscribe_cta", KbSubscribeCTA(d.VKGroupURL))
			_ = d.State.SetStep(ctx, fc.VkID, StepFreeGenStart)
			return
		}
	}

	if fc.User.Gender == "unknown" {
		savedState := *fc.State
		savedState.Step = StepAwaitingGender
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = d.Sender.SendMsg(ctx, fc.VkID, "gender_select", KbGender())
		return
	}

	savedState := *fc.State
	savedState.PhotoURL = ""

	if fc.State.PromptType == "edit" {
		savedState.Step = StepAwaitingPhotoEdit
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = d.Sender.SendMsg(ctx, fc.VkID, "edit_photo_intro", KbBack())
	} else {
		savedState.Step = StepAwaitingPhoto
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
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
		_ = d.Sender.SendMsg(ctx, fc.VkID, "edit_photo_intro", KbBack())
	} else {
		newState.Step = StepAwaitingPhoto
		_ = d.State.Set(ctx, fc.VkID, &newState)
		_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
	}
}

func HandleAwaitingPhoto(ctx context.Context, fc *Context, d *Deps) {
	photos := []string{}
	if fc.Message != nil {
		photos = fc.Message.Photos
	}

	// Если фото не прислали, напоминаем
	if len(photos) == 0 {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
		return
	}

	// Проверяем остаток генераций
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
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
		p, err := d.PromptRepo.GetByID(ctx, fc.State.TemplateID)
		if err == nil && p != nil {
			prompt = p.Prompt
		}
	}

	startGeneration(ctx, fc, d, uploadedURL, prompt, promptType, "")
}

// startGeneration создаёт запись генерации, списывает генерацию и ставит задачу в очередь.
// waitMsg — кастомный текст ожидания; если пустой, используется шаблон generating_wait.
func startGeneration(ctx context.Context, fc *Context, d *Deps, photoURL, prompt, promptType, waitMsg string) {
	model := fc.State.Model
	if model == "" {
		model = fc.User.PrefModel
	}
	if model == "" {
		model = d.DefaultModel
	}

	gen, err := d.GenRepo.Create(ctx, fc.VkID, promptType, prompt, model, &photoURL)
	if err != nil {
		log.Error().Err(err).Msg("не удалось создать запись генерации")
		_ = d.Sender.SendText(ctx, fc.VkID, "❌ Произошла ошибка. Попробуй позже.", KbBack())
		return
	}

	_ = d.UserRepo.DecrementGens(ctx, fc.VkID)
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	if waitMsg == "" {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "generating_wait", "")
	} else {
		_ = d.Sender.SendText(ctx, fc.VkID, waitMsg, "")
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
		_ = d.Sender.SendText(ctx, fc.VkID, "❌ Ошибка запуска генерации. Попробуй позже.", KbMainMenu())
	}
}

func HandleAwaitingPrompt(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || fc.Message.Text == "" {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "custom_prompt_intro", KbBack())
		return
	}

	state := fc.State
	state.Step = StepAwaitingPhoto
	state.CustomPrompt = fc.Message.Text
	_ = d.State.Set(ctx, fc.VkID, state)
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
}

func HandleAwaitingPhotoEdit(ctx context.Context, fc *Context, d *Deps) {
	photos := []string{}
	if fc.Message != nil {
		photos = fc.Message.Photos
	}
	if len(photos) == 0 {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "edit_photo_intro", KbBack())
		return
	}
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
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
	_ = d.Sender.SendText(ctx, fc.VkID, "✏️ Отлично! Теперь опиши, что нужно изменить на фото:", KbBack())
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
	genderStr := "woman"
	if gender == "male" {
		genderStr = "man"
	}
	return fmt.Sprintf("professional portrait photo of a %s, studio lighting, high quality, photorealistic", genderStr)
}

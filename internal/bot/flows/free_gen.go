package flows

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/worker"
)

func HandleFreeGenStart(ctx context.Context, fc *Context, d *Deps) {
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
		_ = d.State.Set(ctx, fc.VkID, &State{Step: StepAwaitingGender, PromptType: promptType})
		_ = d.Sender.SendMsg(ctx, fc.VkID, "gender_select", KbGender())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepAwaitingPhoto, PromptType: promptType})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
}

func HandleGenderSelect(ctx context.Context, fc *Context, d *Deps, gender string) {
	_ = d.UserRepo.SetGender(ctx, fc.VkID, gender)
	fc.User.Gender = gender

	promptType := fc.State.PromptType
	if promptType == "" {
		promptType = "free"
	}
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepAwaitingPhoto, PromptType: promptType})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
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
	promptType := fc.State.PromptType
	prompt := buildDefaultPrompt(fc.User.Gender, promptType)
	if fc.State.TemplateID > 0 {
		p, err := d.PromptRepo.GetByID(ctx, fc.State.TemplateID)
		if err == nil && p != nil {
			prompt = p.Prompt
		}
	}

	model := d.DefaultModel
	if fc.State.Model != "" {
		model = fc.State.Model
	}

	gen, err := d.GenRepo.Create(ctx, fc.VkID, promptType, prompt, model, &photoURL)
	if err != nil {
		log.Error().Err(err).Msg("не удалось создать запись генерации")
		_ = d.Sender.SendText(ctx, fc.VkID, "❌ Произошла ошибка. Попробуй позже.", KbBack())
		return
	}

	_ = d.UserRepo.DecrementGens(ctx, fc.VkID)
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = d.Sender.SendMsg(ctx, fc.VkID, "generating_wait", "")

	payload := worker.GeneratePayload{
		GenerationID: gen.ID,
		UserVKID:     fc.VkID,
		Model:        model,
		Images:       []string{photoURL},
		Prompt:       prompt,
		Resolution:   "1k",
		OutputFormat: "jpeg",
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
	_ = d.State.Set(ctx, fc.VkID, state)
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
}

func HandleAwaitingPhotoEdit(ctx context.Context, fc *Context, d *Deps) {
	HandleAwaitingPhoto(ctx, fc, d)
}

func buildDefaultPrompt(gender, promptType string) string {
	genderStr := "woman"
	if gender == "male" {
		genderStr = "man"
	}
	return fmt.Sprintf("professional portrait photo of a %s, studio lighting, high quality, photorealistic", genderStr)
}

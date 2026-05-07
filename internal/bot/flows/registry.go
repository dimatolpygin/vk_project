package flows

import (
	"context"

	"github.com/rs/zerolog/log"
)

// Registry диспетчеризует события VK на нужный flow.
type Registry struct {
	d *Deps
}

func NewRegistry(d *Deps) *Registry {
	return &Registry{d: d}
}

func (r *Registry) HandleMessage(ctx context.Context, fc *Context) {
	step := fc.State.Step

	switch step {
	case StepWelcome, "":
		HandleWelcome(ctx, fc, r.d)
	case StepAfterGen:
		HandleAfterGen(ctx, fc, r.d)
	case StepAwaitingPhoto:
		HandleAwaitingPhoto(ctx, fc, r.d)
	case StepAwaitingPrompt:
		HandleAwaitingPrompt(ctx, fc, r.d)
	case StepAwaitingPhotoEdit:
		HandleAwaitingPhotoEdit(ctx, fc, r.d)
	default:
		if fc.User.Status == "paid" || fc.User.HasGens() {
			HandleMainMenu(ctx, fc, r.d)
		} else {
			HandleWelcome(ctx, fc, r.d)
		}
	}
}

func (r *Registry) HandleCallback(ctx context.Context, fc *Context) {
	cb := fc.Callback
	if cb == nil {
		return
	}

	log.Info().Int64("vk_id", fc.VkID).Str("cb_type", cb.Type).Msg("обработка callback")

	switch cb.Type {
	case "back":
		HandleBack(ctx, fc, r.d)
	case "free_gen":
		HandleFreeGenStart(ctx, fc, r.d)
	case "check_sub":
		HandleCheckSubscription(ctx, fc, r.d)
	case "gender_male":
		HandleGenderSelect(ctx, fc, r.d, "male")
	case "gender_female":
		HandleGenderSelect(ctx, fc, r.d, "female")
	case "tariffs":
		HandleShowTariffs(ctx, fc, r.d)
	case "buy_tariff":
		HandleBuyTariff(ctx, fc, r.d)
	case "referral":
		HandleReferral(ctx, fc, r.d)
	case "main_menu":
		HandleMainMenu(ctx, fc, r.d)
	case "ready_prompts":
		HandleReadyPromptsMenu(ctx, fc, r.d)
	case "select_category":
		HandleSelectCategory(ctx, fc, r.d)
	case "select_prompt":
		HandleSelectPrompt(ctx, fc, r.d)
	case "custom_prompt":
		HandleCustomPromptStart(ctx, fc, r.d)
	case "edit_photo":
		HandleEditPhotoStart(ctx, fc, r.d)
	case "couple":
		HandleCoupleStart(ctx, fc, r.d)
	case "saved_photo":
		HandleSavedPhotoStart(ctx, fc, r.d)
	case "settings":
		HandleSettings(ctx, fc, r.d)
	case "quality":
		HandleQuality(ctx, fc, r.d)
	case "format":
		HandleFormat(ctx, fc, r.d)
	case "balance":
		HandleBalance(ctx, fc, r.d)
	case "model":
		HandleModel(ctx, fc, r.d)
	case "model_nbp":
		HandleSetModel(ctx, fc, r.d, "google/nano-banana-pro")
	case "model_nb2":
		HandleSetModel(ctx, fc, r.d, "google/nano-banana-2")
	case "model_gpt2":
		HandleSetModel(ctx, fc, r.d, "openai/gpt-image-2")
	case "quality_1k":
		HandleSetResolution(ctx, fc, r.d, "1k")
	case "quality_2k":
		HandleSetResolution(ctx, fc, r.d, "2k")
	case "quality_4k":
		HandleSetResolution(ctx, fc, r.d, "4k")
	case "ar_1_1":
		HandleSetAspectRatio(ctx, fc, r.d, "1:1")
	case "ar_9_16":
		HandleSetAspectRatio(ctx, fc, r.d, "9:16")
	case "ar_16_9":
		HandleSetAspectRatio(ctx, fc, r.d, "16:9")
	case "buy_gens":
		HandleShowTariffs(ctx, fc, r.d)
	case "support":
		HandleSupport(ctx, fc, r.d)
	case "examples":
		HandleExamples(ctx, fc, r.d)
	default:
		log.Warn().Str("type", cb.Type).Msg("неизвестный callback")
		_ = r.d.Sender.SendText(ctx, fc.VkID, "Неизвестная команда. Возвращаю в главное меню.", KbMainMenu())
		_ = r.d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	}
}

package flows

import (
	"context"

	"github.com/rs/zerolog/log"
)

type Registry struct {
	d *Deps
}

func NewRegistry(d *Deps) *Registry {
	return &Registry{d: d}
}

func (r *Registry) HandleMessage(ctx context.Context, fc *Context) {
	switch fc.State.Step {
	case StepWelcome, "":
		HandleWelcome(ctx, fc, r.d)
	case StepAfterGen:
		HandleAfterGen(ctx, fc, r.d)
	case StepReadyPromptsCategories, StepReadyPromptsPrompts:
		HandleReadyPromptsBrowse(ctx, fc, r.d)
	case StepCoupleAwaitingPhoto:
		HandleCoupleAwaitingPhoto(ctx, fc, r.d)
	case StepCoupleCategories, StepCouplePrompts:
		HandleCoupleBrowse(ctx, fc, r.d)
	case StepKidsCategories, StepKidsPrompts, StepGreetingsCategories, StepGreetingsPrompts,
		StepTrendsCategories, StepTrendsPrompts:
		HandleSectionBrowse(ctx, fc, r.d)
	case StepAwaitingPhoto:
		HandleAwaitingPhoto(ctx, fc, r.d)
	case StepAwaitingSavedPhoto:
		HandleSavedPhotoReceived(ctx, fc, r.d)
	case StepAwaitingPrompt:
		HandleAwaitingPrompt(ctx, fc, r.d)
	case StepAwaitingPhotoEdit:
		HandleAwaitingPhotoEdit(ctx, fc, r.d)
	case StepAwaitingEditPrompt, StepAwaitingResultEdit:
		HandleResultEditPrompt(ctx, fc, r.d)
	default:
		if fc.User.Status == "paid" || fc.User.HasGens() {
			HandleMainMenu(ctx, fc, r.d)
		} else {
			HandleWelcome(ctx, fc, r.d)
		}
	}
}

func (r *Registry) HandleCallback(ctx context.Context, fc *Context) {
	if fc.Callback == nil {
		return
	}

	log.Info().Int64("vk_id", fc.VkID).Str("cb_type", fc.Callback.Type).Msg("обработка callback")

	switch fc.Callback.Type {
	case "back":
		HandleBack(ctx, fc, r.d)
	case "free_gen":
		HandleFreeGenStart(ctx, fc, r.d)
	case "gen_again":
		HandleGenAgain(ctx, fc, r.d)
	case "check_sub":
		HandleCheckSubscription(ctx, fc, r.d)
	case "gender_male":
		HandleGenderSelect(ctx, fc, r.d, "male")
	case "gender_female":
		HandleGenderSelect(ctx, fc, r.d, "female")
	case "tariffs", "buy_gens":
		HandleShowTariffs(ctx, fc, r.d)
	case "buy_tariff":
		HandleBuyTariff(ctx, fc, r.d)
	case "broadcast_cta":
		HandleBroadcastCTA(ctx, fc, r.d)
	case "referral":
		HandleReferral(ctx, fc, r.d)
	case "main_menu":
		HandleMainMenu(ctx, fc, r.d)
	case "ready_prompts":
		HandleReadyPromptsMenu(ctx, fc, r.d)
	case "ready_prompts_page":
		HandleReadyPromptsPage(ctx, fc, r.d)
	case "select_category", "open_category":
		HandleOpenCategory(ctx, fc, r.d)
	case "open_category_page":
		HandleOpenCategoryPage(ctx, fc, r.d)
	case "prompts_page":
		HandlePromptsPage(ctx, fc, r.d)
	case "select_prompt":
		HandleSelectPrompt(ctx, fc, r.d)
	case "custom_prompt":
		HandleCustomPromptStart(ctx, fc, r.d)
	case "edit_photo":
		HandleEditPhotoStart(ctx, fc, r.d)
	case "edit_result":
		HandleEditResultStart(ctx, fc, r.d)
	case "couple":
		HandleCoupleStart(ctx, fc, r.d)
	case "kids":
		HandleKidsMenu(ctx, fc, r.d)
	case "kids_page":
		HandleKidsPage(ctx, fc, r.d)
	case "greetings":
		HandleGreetingsMenu(ctx, fc, r.d)
	case "greetings_page":
		HandleGreetingsPage(ctx, fc, r.d)
	case "trends":
		HandleTrendsMenu(ctx, fc, r.d)
	case "trends_page":
		HandleTrendsPage(ctx, fc, r.d)
	case "couple_page":
		HandleCouplePage(ctx, fc, r.d)
	case "saved_photo":
		HandleSavedPhotoStart(ctx, fc, r.d)
	case "saved_photo_upload":
		HandleSavedPhotoUploadStart(ctx, fc, r.d)
	case "toggle_saved_photo":
		HandleToggleSavedPhoto(ctx, fc, r.d)
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
	case "support":
		HandleSupport(ctx, fc, r.d)
	case "examples":
		HandleExamples(ctx, fc, r.d)
	case "examples_self", "examples_couple", "examples_kids", "examples_edit", "examples_greetings", "examples_misc":
		HandleExampleCategory(ctx, fc, r.d, fc.Callback.Type)
	default:
		log.Warn().Str("type", fc.Callback.Type).Msg("неизвестный callback")
		_ = sendScreen(ctx, r.d, fc.VkID, "unknown_command", ScreenOptions{})
		_ = r.d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	}
}

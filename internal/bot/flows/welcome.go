package flows

import (
	"context"

	"github.com/rs/zerolog/log"
)

func HandleWelcome(ctx context.Context, fc *Context, d *Deps) {
	if err := sendScreen(ctx, d, fc.VkID, "welcome", ScreenOptions{}); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("РѕС€РёР±РєР° РѕС‚РїСЂР°РІРєРё welcome")
	}
	_ = d.State.SetStep(ctx, fc.VkID, StepWelcome)
}

func HandleBack(ctx context.Context, fc *Context, d *Deps) {
	if fc.State.Step == StepReadyPromptsPrompts {
		if err := showReadyPromptsCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("РѕС€РёР±РєР° РІРѕР·РІСЂР°С‚Р° Рє РєР°С‚РµРіРѕСЂРёСЏРј РіРѕС‚РѕРІС‹С… РїСЂРѕРјС‚РѕРІ")
		}
		return
	}

	if fc.State.Step == StepCouplePrompts {
		if err := showCoupleCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("РѕС€РёР±РєР° РІРѕР·РІСЂР°С‚Р° Рє РїР°СЂРЅС‹Рј РєР°С‚РµРіРѕСЂРёСЏРј")
		}
		return
	}

	if fc.State.Step == StepReadyPromptsCategories || fc.State.Step == StepCoupleCategories {
		if fc.User.Status == "paid" || fc.User.HasGens() {
			HandleMainMenu(ctx, fc, d)
		} else {
			HandleWelcome(ctx, fc, d)
		}
		return
	}

	if fc.State.Step == StepSettings {
		if fc.User.Status == "paid" || fc.User.HasGens() {
			HandleMainMenu(ctx, fc, d)
		} else {
			HandleWelcome(ctx, fc, d)
		}
		return
	}

	if fc.State.Step == StepTariffs && fc.State.PrevStep == StepSettings {
		HandleSettings(ctx, fc, d)
		return
	}

	if fc.State.Step == StepTariffs && fc.State.PrevStep == StepAfterGen && fc.State.PhotoURL != "" {
		state := *fc.State
		state.Step = StepAfterGen
		_ = d.State.Set(ctx, fc.VkID, &state)
		_ = showAfterGenScreen(ctx, fc, d, fc.State.PhotoURL)
		return
	}

	if fc.User.Status == "paid" || fc.User.HasGens() {
		HandleMainMenu(ctx, fc, d)
	} else {
		HandleWelcome(ctx, fc, d)
	}
}

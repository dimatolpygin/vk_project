package flows

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

func HandleWelcome(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message != nil && strings.HasPrefix(fc.Message.Text, "/start ") {
		refCode := strings.TrimPrefix(fc.Message.Text, "/start ")
		if refCode != "" {
			referrer, err := d.UserRepo.GetByReferralCode(ctx, refCode)
			if err == nil && referrer != nil && referrer.VKID != fc.VkID {
				_ = d.RefRepo.Create(ctx, referrer.VKID, fc.VkID)
				trackEvent(ctx, d, fc.VkID, repository.ActivityEventReferralCreated, "referral", "welcome", map[string]any{
					"referrer_vk_id": referrer.VKID,
				})
			}
		}
	}

	if err := sendScreen(ctx, d, fc.VkID, "welcome", ScreenOptions{}); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка отправки welcome")
	}
	_ = d.State.SetStep(ctx, fc.VkID, StepWelcome)
}

func HandleBack(ctx context.Context, fc *Context, d *Deps) {
	if fc.State.Step == StepReadyPromptsPrompts {
		if err := showReadyPromptsCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("ошибка возврата к категориям готовых промтов")
		}
		return
	}

	if fc.State.Step == StepCouplePrompts {
		if err := showCoupleCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("ошибка возврата к парным категориям")
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

package flows

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
)

func HandleWelcome(ctx context.Context, fc *Context, d *Deps) {
	// Обрабатываем реферальный код из payload стартового сообщения
	if fc.Message != nil && strings.HasPrefix(fc.Message.Text, "/start ") {
		refCode := strings.TrimPrefix(fc.Message.Text, "/start ")
		if refCode != "" {
			referrer, err := d.UserRepo.GetByReferralCode(ctx, refCode)
			if err == nil && referrer != nil && referrer.VKID != fc.VkID {
				_ = d.RefRepo.Create(ctx, referrer.VKID, fc.VkID)
			}
		}
	}

	if err := d.Sender.SendMsg(ctx, fc.VkID, "welcome", KbWelcome()); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка отправки welcome")
	}
	_ = d.State.SetStep(ctx, fc.VkID, StepWelcome)
}

func HandleBack(ctx context.Context, fc *Context, d *Deps) {
	if fc.User.Status == "paid" || fc.User.HasGens() {
		HandleMainMenu(ctx, fc, d)
	} else {
		HandleWelcome(ctx, fc, d)
	}
}

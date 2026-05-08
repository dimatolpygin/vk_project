package flows

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/yukassa"
)

func HandleShowTariffs(ctx context.Context, fc *Context, d *Deps) {
	tariffs, err := d.TariffRepo.ListActive(ctx)
	if err != nil || len(tariffs) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:     StepTariffs,
		PrevStep: fc.State.Step,
		PhotoURL: fc.State.PhotoURL,
	})
	_ = sendScreen(ctx, d, fc.VkID, "tariffs", ScreenOptions{PrefixRows: tariffRows(tariffs)})
}

func HandleBuyTariff(ctx context.Context, fc *Context, d *Deps) {
	tariffID := fc.Callback.TariffID
	if tariffID == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "payment_invalid_tariff", ScreenOptions{})
		return
	}

	tariff, err := d.TariffRepo.GetByID(ctx, tariffID)
	if err != nil || tariff == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_tariff_not_found", ScreenOptions{})
		return
	}

	order, err := d.OrderRepo.Create(ctx, fc.VkID, tariffID, tariff.Price)
	if err != nil {
		log.Error().Err(err).Msg("не удалось создать заказ")
		_ = sendScreen(ctx, d, fc.VkID, "payment_order_error", ScreenOptions{})
		return
	}

	payment, err := d.Yukassa.CreatePayment(ctx, yukassa.PaymentRequest{
		Amount:    tariff.Price,
		TariffID:  tariffID,
		UserVKID:  fc.VkID,
		OrderID:   order.ID,
		ReturnURL: fmt.Sprintf("%s/vk/return", d.BotWebhookURL),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания платежа ЮKassa")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	_ = d.OrderRepo.SetPaymentID(ctx, order.ID, payment.PaymentID)
	_ = sendScreen(ctx, d, fc.VkID, "payment_checkout", ScreenOptions{
		Data: map[string]any{
			"TariffName":  tariff.Name,
			"Description": tariff.Description,
			"Price":       fmt.Sprintf("%.0f₽", tariff.Price),
		},
		Links: map[string]string{"payment_url": payment.PaymentURL},
	})
}

func ProcessSuccessfulPayment(ctx context.Context, d *Deps, paymentID string, userVKID int64, tariffID int) error {
	if err := d.OrderRepo.SetStatus(ctx, paymentID, "succeeded"); err != nil {
		return err
	}

	tariff, err := d.TariffRepo.GetByID(ctx, tariffID)
	if err != nil || tariff == nil {
		return fmt.Errorf("тариф %d не найден", tariffID)
	}

	if err := d.UserRepo.AddPaidGens(ctx, userVKID, tariff.GensCount); err != nil {
		return err
	}
	trackEvent(ctx, d, userVKID, repository.ActivityEventPaymentSucceeded, "buy_tariff", "payment_success", map[string]any{
		"payment_id": paymentID,
		"tariff_id":  tariffID,
		"gens_count": tariff.GensCount,
		"amount":     tariff.Price,
	})

	ref, err := d.RefRepo.GiveBonus(ctx, userVKID)
	if err == nil && ref != nil {
		_ = d.UserRepo.AddFreeGens(ctx, ref.ReferrerVKID, 2)
		trackEvent(ctx, d, ref.ReferrerVKID, repository.ActivityEventReferralBonusAwarded, "referral", "referral_bonus_awarded", map[string]any{
			"referred_vk_id": userVKID,
			"bonus_gens":     2,
		})
		_ = sendScreen(ctx, d, ref.ReferrerVKID, "referral_bonus_awarded", ScreenOptions{})
	}

	_ = sendScreen(ctx, d, userVKID, "payment_success", ScreenOptions{})
	return nil
}

package flows

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/yukassa"
)

func HandleShowTariffs(ctx context.Context, fc *Context, d *Deps) {
	tariffs, err := d.TariffRepo.ListActive(ctx)
	if err != nil || len(tariffs) == 0 {
		_ = d.Sender.SendText(ctx, fc.VkID, "Тарифы временно недоступны. Попробуй позже.", KbBack())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:     StepTariffs,
		PrevStep: fc.State.Step,
		PhotoURL: fc.State.PhotoURL,
	})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "tariffs", KbTariffs(tariffs))
}

func HandleBuyTariff(ctx context.Context, fc *Context, d *Deps) {
	tariffID := fc.Callback.TariffID
	if tariffID == 0 {
		_ = d.Sender.SendText(ctx, fc.VkID, "Неверный тариф.", KbBack())
		return
	}

	tariff, err := d.TariffRepo.GetByID(ctx, tariffID)
	if err != nil || tariff == nil {
		_ = d.Sender.SendText(ctx, fc.VkID, "Тариф не найден.", KbBack())
		return
	}

	order, err := d.OrderRepo.Create(ctx, fc.VkID, tariffID, tariff.Price)
	if err != nil {
		log.Error().Err(err).Msg("не удалось создать заказ")
		_ = d.Sender.SendText(ctx, fc.VkID, "Ошибка создания заказа. Попробуй позже.", KbBack())
		return
	}

	payResp, err := d.Yukassa.CreatePayment(ctx, yukassa.PaymentRequest{
		Amount:    tariff.Price,
		TariffID:  tariffID,
		UserVKID:  fc.VkID,
		OrderID:   order.ID,
		ReturnURL: fmt.Sprintf("%s/vk/return", d.BotWebhookURL),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания платежа ЮKassa")
		_ = d.Sender.SendText(ctx, fc.VkID, "Не удалось создать ссылку на оплату. Попробуй позже.", KbBack())
		return
	}

	_ = d.OrderRepo.SetPaymentID(ctx, order.ID, payResp.PaymentID)

	text := fmt.Sprintf("💳 *%s*\n%s\n\nСумма: %.0f₽\n\nДля оплаты перейди по ссылке ниже:",
		tariff.Name, tariff.Description, tariff.Price)

	payKb := paymentLinkKb(payResp.PaymentURL)
	_ = d.Sender.SendText(ctx, fc.VkID, text, payKb)
}

func paymentLinkKb(url string) string {
	kb := &Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "open_link", Label: "💳 Перейти к оплате", Link: url}}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}},
	}}
	return kbJSON(kb)
}

// ProcessSuccessfulPayment вызывается из webhook-обработчика ЮKassa.
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

	// Реферальный бонус
	ref, err := d.RefRepo.GiveBonus(ctx, userVKID)
	if err == nil && ref != nil {
		_ = d.UserRepo.AddFreeGens(ctx, ref.ReferrerVKID, 2)
		_ = d.Sender.SendText(ctx, ref.ReferrerVKID, "🎁 Твой реферал оплатил тариф! +2 бесплатные генерации.", KbMainMenu())
	}

	_ = d.Sender.SendMsg(ctx, userVKID, "payment_success", KbMainMenu())
	return nil
}

package flows

import (
	"context"
	"fmt"
)

func HandleSettings(ctx context.Context, fc *Context, d *Deps) {
	totalGens := fc.User.FreeGens + fc.User.PaidGens
	model := d.DefaultModel
	if fc.State.Model != "" {
		model = fc.State.Model
	}

	text := fmt.Sprintf("⚙️ Настройки\n\n"+
		"👤 Пол: %s\n"+
		"🎯 Генераций осталось: %d (бесплатных: %d, платных: %d)\n"+
		"🤖 Модель: %s\n\n"+
		"💳 Пополнить баланс:",
		fc.User.Gender, totalGens, fc.User.FreeGens, fc.User.PaidGens, model)

	tariffs, _ := d.TariffRepo.ListActive(ctx)
	kb := KbTariffs(tariffs)
	_ = d.Sender.SendText(ctx, fc.VkID, text, kb)
}

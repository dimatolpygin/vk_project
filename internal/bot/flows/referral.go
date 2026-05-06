package flows

import (
	"context"
	"fmt"
)

func HandleReferral(ctx context.Context, fc *Context, d *Deps) {
	count, _ := d.RefRepo.CountByReferrer(ctx, fc.VkID)
	refLink := fmt.Sprintf("https://vk.com/app%s_-?ref=%s", "REPLACE_WITH_APP_ID", fc.User.ReferralCode)

	text := fmt.Sprintf("🎁 Реферальная программа\n\n"+
		"Ты пригласил уже %d человек.\n\n"+
		"За каждого друга, который оплатит тариф, ты получишь 2 бесплатные генерации!\n\n"+
		"Твоя ссылка:\n%s", count, refLink)

	_ = d.Sender.SendText(ctx, fc.VkID, text, KbBack())
}

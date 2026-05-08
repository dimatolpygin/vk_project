package flows

import (
	"context"
	"fmt"
)

func HandleReferral(ctx context.Context, fc *Context, d *Deps) {
	count, _ := d.RefRepo.CountByReferrer(ctx, fc.VkID)
	refLink := fmt.Sprintf("https://vk.com/app%s_-?ref=%s", "REPLACE_WITH_APP_ID", fc.User.ReferralCode)

	_ = sendScreen(ctx, d, fc.VkID, "referral_status", ScreenOptions{
		Data: map[string]any{
			"Count":   count,
			"RefLink": refLink,
		},
	})
}

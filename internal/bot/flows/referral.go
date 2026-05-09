package flows

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func HandleReferral(ctx context.Context, fc *Context, d *Deps) {
	count, _ := d.RefRepo.CountByReferrer(ctx, fc.VkID)
	refLink := buildReferralLink(d.VKGroupID, fc.User.ReferralCode)

	_ = sendScreen(ctx, d, fc.VkID, "referral_status", ScreenOptions{
		Data: map[string]any{
			"Count":   count,
			"RefLink": refLink,
		},
	})
}

func buildReferralLink(groupID int64, referralCode string) string {
	referralCode = strings.TrimSpace(referralCode)
	if groupID <= 0 || referralCode == "" {
		return ""
	}
	return fmt.Sprintf("https://vk.me/club%d?ref=%s", groupID, url.QueryEscape(referralCode))
}

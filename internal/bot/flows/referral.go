package flows

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func HandleReferral(ctx context.Context, fc *Context, d *Deps) {
	count, refLink := referralStatus(ctx, fc, d)

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

func referralStatus(ctx context.Context, fc *Context, d *Deps) (int, string) {
	count := 0
	if d.RefRepo != nil {
		count, _ = d.RefRepo.CountByReferrer(ctx, fc.VkID)
	}

	referralCode := ""
	if fc != nil && fc.User != nil {
		referralCode = fc.User.ReferralCode
	}

	return count, buildReferralLink(d.VKGroupID, referralCode)
}

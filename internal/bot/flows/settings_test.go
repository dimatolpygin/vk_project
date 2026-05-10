package flows

import (
	"context"
	"strings"
	"testing"
)

func TestHandleSettingsDisplaysTotalBalance(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
	}
	fc := &Context{
		VkID:  501,
		User:  &User{FreeGens: 2, PaidGens: 3},
		State: &State{},
	}

	HandleSettings(context.Background(), fc, deps)

	if len(sender.screens) == 0 {
		t.Fatal("expected settings screen to be sent")
	}
	if got := sender.screens[len(sender.screens)-1].Text; !strings.Contains(got, "Баланс генераций: 5") {
		t.Fatalf("expected total balance in settings screen, got %q", got)
	}
	if got := sender.screens[len(sender.screens)-1].Text; !strings.Contains(got, "ID: 501") {
		t.Fatalf("expected user id in settings screen, got %q", got)
	}
	if got := sender.screens[len(sender.screens)-1].Text; !strings.Contains(got, "Приглашено рефералов: 0") {
		t.Fatalf("expected referral count in settings screen, got %q", got)
	}
}

func TestHandleBalanceDisplaysUnifiedBalance(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
	}
	fc := &Context{
		VkID:  502,
		User:  &User{FreeGens: 2, PaidGens: 3},
		State: &State{},
	}

	HandleBalance(context.Background(), fc, deps)

	if len(sender.screens) == 0 {
		t.Fatal("expected balance screen to be sent")
	}
	got := sender.screens[len(sender.screens)-1].Text
	if !strings.Contains(got, "Доступно генераций: 5") {
		t.Fatalf("expected unified balance in balance screen, got %q", got)
	}
	if strings.Contains(got, "Бесплатных") || strings.Contains(got, "Платных") {
		t.Fatalf("expected split balances to be hidden, got %q", got)
	}
}

func TestHandleSettingsDisplaysReferralLinkWithoutExtraButton(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender:    sender,
		State:     stateMgr,
		VKGroupID: 229805415,
	}
	fc := &Context{
		VkID:  503,
		User:  &User{FreeGens: 1, PaidGens: 2, ReferralCode: "ref_abc123"},
		State: &State{},
	}

	HandleSettings(context.Background(), fc, deps)

	if len(sender.screens) == 0 {
		t.Fatal("expected settings screen to be sent")
	}

	screen := sender.screens[len(sender.screens)-1]
	if !strings.Contains(screen.Text, "https://vk.me/club229805415?ref=ref_abc123") {
		t.Fatalf("expected referral link in settings screen, got %q", screen.Text)
	}
	if strings.Contains(screen.Keyboard, "Рефералы") {
		t.Fatalf("expected referral button to be removed from settings keyboard, got %q", screen.Keyboard)
	}
}

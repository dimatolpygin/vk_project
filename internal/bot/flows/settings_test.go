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

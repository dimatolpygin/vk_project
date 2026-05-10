package repository

import "testing"

func TestUpgradedDefaultMessageTextReplacesKnownLegacyDefaults(t *testing.T) {
	got := upgradedDefaultMessageText(
		"payment_success",
		"🎉 Оплата прошла успешно!\n\nДобро пожаловать в мир нейрофотосессий! Твои генерации зачислены.",
	)

	if got == "🎉 Оплата прошла успешно!\n\nДобро пожаловать в мир нейрофотосессий! Твои генерации зачислены." {
		t.Fatalf("expected legacy payment_success text to be upgraded, got %q", got)
	}
	if got == "" {
		t.Fatal("expected upgraded text to be non-empty")
	}
}

func TestUpgradedDefaultMessageTextPreservesCustomText(t *testing.T) {
	const custom = "Кастомный текст пользователя"

	if got := upgradedDefaultMessageText("payment_success", custom); got != custom {
		t.Fatalf("expected custom text to stay unchanged, got %q", got)
	}
}

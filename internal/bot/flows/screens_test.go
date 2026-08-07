package flows

import (
	"context"
	"strings"
	"testing"

	"vk_neuro_bot/internal/repository"
)

// Пустой текст экрана в БД — причина, по которой bottom_menu пять дней подряд
// отбивался ВК с ошибкой 100. Экран обязан уйти с каноническим текстом.
func TestSendScreenSubstitutesDefaultTextWhenStoredTextIsEmpty(t *testing.T) {
	sender := &fakeSender{}
	deps := &Deps{
		Sender:  sender,
		MsgRepo: &fakeMessageReader{msg: &repository.Message{Key: "bottom_menu", Text: "   "}},
	}

	if err := sendScreen(context.Background(), deps, 42, "bottom_menu", ScreenOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.screens) != 1 {
		t.Fatalf("expected the screen to be sent, got %d", len(sender.screens))
	}
	if got := sender.screens[0].Text; got != "Быстрое меню" {
		t.Fatalf("expected canonical text, got %q", got)
	}
}

// Если канонического текста тоже нет, отправлять нечего: молчаливый отказ ВК
// выглядит для пользователя как сломанная кнопка, поэтому падаем явно.
func TestSendScreenRefusesTrulyEmptyScreen(t *testing.T) {
	sender := &fakeSender{}
	deps := &Deps{
		Sender:  sender,
		MsgRepo: &fakeMessageReader{msg: &repository.Message{Key: "no_such_screen", Text: ""}},
	}

	err := sendScreen(context.Background(), deps, 42, "no_such_screen", ScreenOptions{})
	if err == nil {
		t.Fatal("expected an error for a screen without text and image")
	}
	if len(sender.screens) != 0 {
		t.Fatalf("nothing should be sent, got %d screens", len(sender.screens))
	}
}

// Экран с картинкой без текста — законный случай, его трогать нельзя.
func TestSendScreenAllowsImageOnlyScreen(t *testing.T) {
	sender := &fakeSender{}
	imageURL := "https://example.com/collage.jpg"
	deps := &Deps{
		Sender:  sender,
		MsgRepo: &fakeMessageReader{msg: &repository.Message{Key: "examples_self", Text: "", ImageURL: &imageURL}},
	}

	if err := sendScreen(context.Background(), deps, 42, "examples_self", ScreenOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.screens) != 1 || sender.screens[0].Text != "" {
		t.Fatalf("expected image-only screen to pass through untouched, got %#v", sender.screens)
	}
}

func TestTariffRowsDropGenerationCountSuffix(t *testing.T) {
	rows := TariffRows([]*repository.Tariff{
		{ID: 3, Name: "5 фотогенераций", Price: 139, GensCount: 5},
	})

	if len(rows) != 1 || len(rows[0]) != 1 {
		t.Fatalf("expected a single tariff row, got %#v", rows)
	}
	label := rows[0][0].Action.Label
	if label != "💳 5 фотогенераций — 139₽" {
		t.Fatalf("unexpected tariff label: %q", label)
	}
	if strings.Contains(label, "ген.") || strings.Contains(label, "шт") {
		t.Fatalf("tariff label must not repeat the generation count: %q", label)
	}
}

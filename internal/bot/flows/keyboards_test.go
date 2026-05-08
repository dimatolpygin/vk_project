package flows

import (
	"encoding/json"
	"strings"
	"testing"

	"vk_neuro_bot/internal/content"
)

func TestRenderContentKeyboardSupportsKinds(t *testing.T) {
	cfg := content.Keyboard{
		Inline: true,
		Items: []content.Button{
			{SlotID: "static", ActionKey: "main_menu", Kind: content.ButtonKindCallback, Label: "Главное меню", Visible: true, Color: "primary", Row: 0, Position: 0},
			{SlotID: "select", ActionKey: "quality_2k", Kind: content.ButtonKindSelect, Label: "HD (2k)", Value: "2k", Visible: true, Color: "primary", Row: 1, Position: 0},
			{SlotID: "toggle", ActionKey: "toggle_saved_photo", Kind: content.ButtonKindToggle, Label: "Выкл", LabelOn: "Вкл", Visible: true, Color: "secondary", Row: 2, Position: 0},
			{SlotID: "link", ActionKey: "download_photo", Kind: content.ButtonKindOpenLink, Label: "Скачать", LinkBinding: "download_photo", Visible: true, Row: 3, Position: 0},
		},
	}

	raw := RenderContentKeyboard(cfg, KeyboardRenderOptions{
		SelectedValue: "2k",
		ToggleOn:      true,
		Links:         map[string]string{"download_photo": "https://example.com/photo.png"},
	})

	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal rendered keyboard: %v", err)
	}

	if got := keyboard.Buttons[0][0].Action.Label; got != "Главное меню" {
		t.Fatalf("unexpected callback label: %q", got)
	}
	if got := keyboard.Buttons[1][0].Action.Label; !strings.HasPrefix(got, "✅ ") {
		t.Fatalf("expected selected option to be marked, got %q", got)
	}
	if got := keyboard.Buttons[2][0].Action.Label; got != "Вкл" {
		t.Fatalf("expected toggle-on label, got %q", got)
	}
	if got := keyboard.Buttons[3][0].Action.Link; got != "https://example.com/photo.png" {
		t.Fatalf("unexpected open_link URL: %q", got)
	}
}

func TestRenderContentKeyboardWithRowsKeepsPrefixRows(t *testing.T) {
	def, ok := content.Definition("tariffs")
	if !ok {
		t.Fatal("tariffs definition not found")
	}

	prefix := [][]KbBtn{{
		{Action: KbAction{Type: "callback", Label: "Тариф 1", Payload: cbPayload("buy_tariff")}, Color: "primary"},
	}}

	raw := RenderContentKeyboardWithRows(def.Keyboard, prefix, KeyboardRenderOptions{})

	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal rendered keyboard: %v", err)
	}

	if got := keyboard.Buttons[0][0].Action.Label; got != "Тариф 1" {
		t.Fatalf("expected prefix row first, got %q", got)
	}
	lastRow := keyboard.Buttons[len(keyboard.Buttons)-1]
	if got := lastRow[0].Action.Payload; got != cbPayload("back") {
		t.Fatalf("expected service row with back button, got %q", got)
	}
}

func TestKbBottomMenuUsesContentDefinition(t *testing.T) {
	raw := KbBottomMenu()

	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal rendered keyboard: %v", err)
	}

	if keyboard.Inline {
		t.Fatal("expected persistent reply keyboard to be non-inline")
	}
	if len(keyboard.Buttons) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(keyboard.Buttons))
	}
	if got := keyboard.Buttons[0][0].Action.Type; got != "text" {
		t.Fatalf("expected text action type, got %q", got)
	}
	if got := keyboard.Buttons[1][2].Action.Label; got != "🖼 Примеры работ" {
		t.Fatalf("unexpected examples label: %q", got)
	}
}

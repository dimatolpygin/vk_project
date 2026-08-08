package flows

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"vk_neuro_bot/internal/content"
)

func TestRenderContentKeyboardSupportsKinds(t *testing.T) {
	cfg := content.Keyboard{
		Inline: true,
		Items: []content.Button{
			{SlotID: "static", ActionKey: "main_menu", Kind: content.ButtonKindCallback, Label: "Р“Р»Р°РІРЅРѕРµ РјРµРЅСЋ", Visible: true, Color: "primary", Row: 0, Position: 0},
			{SlotID: "select", ActionKey: "quality_2k", Kind: content.ButtonKindSelect, Label: "HD (2k)", Value: "2k", Visible: true, Color: "primary", Row: 1, Position: 0},
			{SlotID: "toggle", ActionKey: "toggle_saved_photo", Kind: content.ButtonKindToggle, Label: "Р’С‹РєР»", LabelOn: "Р’РєР»", Visible: true, Color: "secondary", Row: 2, Position: 0},
			{SlotID: "link", ActionKey: "download_photo", Kind: content.ButtonKindOpenLink, Label: "РЎРєР°С‡Р°С‚СЊ", LinkBinding: "download_photo", Visible: true, Row: 3, Position: 0},
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

	if got := keyboard.Buttons[0][0].Action.Label; got != "Р“Р»Р°РІРЅРѕРµ РјРµРЅСЋ" {
		t.Fatalf("unexpected callback label: %q", got)
	}
	if got := keyboard.Buttons[1][0].Action.Label; !strings.HasPrefix(got, "✅ ") {
		t.Fatalf("expected selected option to be marked, got %q", got)
	}
	if got := keyboard.Buttons[2][0].Action.Label; got != "Р’РєР»" {
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
		{Action: KbAction{Type: "callback", Label: "РўР°СЂРёС„ 1", Payload: cbPayload("buy_tariff")}, Color: "primary"},
	}}

	raw := RenderContentKeyboardWithRows(def.Keyboard, prefix, KeyboardRenderOptions{})

	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal rendered keyboard: %v", err)
	}

	if got := keyboard.Buttons[0][0].Action.Label; got != "РўР°СЂРёС„ 1" {
		t.Fatalf("expected prefix row first, got %q", got)
	}
	lastRow := keyboard.Buttons[len(keyboard.Buttons)-1]
	if got := lastRow[0].Action.Payload; got != cbPayload("back") {
		t.Fatalf("expected service row with back button, got %q", got)
	}
}

func TestRenderContentKeyboardFitsVKInlineRowLimit(t *testing.T) {
	def, ok := content.Definition("examples_collage")
	if !ok {
		t.Fatal("examples_collage definition not found")
	}

	raw := RenderContentKeyboard(def.Keyboard, KeyboardRenderOptions{})

	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal rendered keyboard: %v", err)
	}
	if len(keyboard.Buttons) > vkInlineMaxRows {
		t.Fatalf("examples menu renders %d rows, VK allows %d", len(keyboard.Buttons), vkInlineMaxRows)
	}
	if got := keyboard.Buttons[0][0].Action.Label; got != "🙋 Фото для себя" {
		t.Fatalf("expected «Фото для себя» first, got %q", got)
	}
	lastRow := keyboard.Buttons[len(keyboard.Buttons)-1]
	if got := lastRow[len(lastRow)-1].Action.Payload; got != cbPayload("back") {
		t.Fatalf("expected back button last, got %q", got)
	}
}

func TestFitInlineRowsMergesOverflowingRows(t *testing.T) {
	rows := make([][]KbBtn, 0, 9)
	for i := 0; i < 9; i++ {
		rows = append(rows, []KbBtn{{Action: KbAction{Type: "callback", Label: string(rune('a' + i))}}})
	}

	fitted := fitInlineRows(rows)

	if len(fitted) > vkInlineMaxRows {
		t.Fatalf("expected at most %d rows, got %d", vkInlineMaxRows, len(fitted))
	}

	var flattened []string
	for _, row := range fitted {
		for _, btn := range row {
			flattened = append(flattened, btn.Action.Label)
		}
	}
	if len(flattened) != 9 {
		t.Fatalf("expected all 9 buttons to survive, got %d", len(flattened))
	}
	if flattened[0] != "a" || flattened[8] != "i" {
		t.Fatalf("expected original button order, got %v", flattened)
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

func TestBuildPaginatedRowsUsesSingleColumn(t *testing.T) {
	buttons := []KbBtn{
		{Action: KbAction{Label: "1"}},
		{Action: KbAction{Label: "2"}},
		{Action: KbAction{Label: "3"}},
		{Action: KbAction{Label: "4"}},
		{Action: KbAction{Label: "5"}},
		{Action: KbAction{Label: "6"}},
	}

	rows, page, total := buildPaginatedRows(buttons, 1, "ready_prompts_page", nil)

	if page != 1 || total != 2 {
		t.Fatalf("expected page=1 total=2, got page=%d total=%d", page, total)
	}
	// Первая страница: четыре кнопки по одной в ряд + ряд листалки.
	// Кнопки «Назад» в нём нет — листать назад с первой страницы некуда.
	if len(rows) != 5 {
		t.Fatalf("expected 4 content rows plus the pager row, got %d", len(rows))
	}
	for i, row := range rows[:4] {
		if len(row) != 1 {
			t.Fatalf("expected one button per content row, row %d has %d", i, len(row))
		}
	}
	if got := rows[0][0].Action.Label; got != "1" {
		t.Fatalf("expected the list to start from the first button, got %q", got)
	}
	if len(rows[4]) != 1 || rows[4][0].Action.Label != "Вперёд ➡️" {
		t.Fatalf("expected a forward-only pager row at the bottom, got %#v", rows[4])
	}
}

func TestBuildPaginatedRowsKeepsPagerButtonsInOneBottomRow(t *testing.T) {
	buttons := make([]KbBtn, 0, 12)
	for i := 1; i <= 12; i++ {
		buttons = append(buttons, KbBtn{Action: KbAction{Label: strconv.Itoa(i)}})
	}

	middleRows, middlePage, middleTotal := buildPaginatedRows(buttons, 2, "ready_prompts_page", nil)
	if middlePage != 2 || middleTotal != 3 {
		t.Fatalf("expected middle page=2 total=3, got page=%d total=%d", middlePage, middleTotal)
	}
	// Пять строк: четыре кнопки контента и общий ряд листалки в две колонки.
	if len(middleRows) != 5 {
		t.Fatalf("expected 5 rows on a middle page, got %d", len(middleRows))
	}
	pager := middleRows[4]
	if len(pager) != 2 {
		t.Fatalf("expected both pager buttons in one row, got %#v", pager)
	}
	if pager[0].Action.Label != "⬅️ Назад" || pager[1].Action.Label != "Вперёд ➡️" {
		t.Fatalf("unexpected pager row layout: %#v", pager)
	}

	lastRows, lastPage, lastTotal := buildPaginatedRows(buttons, 3, "ready_prompts_page", nil)
	if lastPage != 3 || lastTotal != 3 {
		t.Fatalf("expected last page=3 total=3, got page=%d total=%d", lastPage, lastTotal)
	}
	lastPager := lastRows[len(lastRows)-1]
	if len(lastPager) != 1 || lastPager[0].Action.Label != "⬅️ Назад" {
		t.Fatalf("last page must offer only the backward pager, got %#v", lastPager)
	}
}

func TestBuildPaginatedRowsCarriesPromptPagerCategoryID(t *testing.T) {
	buttons := []KbBtn{
		{Action: KbAction{Label: "1"}},
		{Action: KbAction{Label: "2"}},
		{Action: KbAction{Label: "3"}},
		{Action: KbAction{Label: "4"}},
		{Action: KbAction{Label: "5"}},
	}

	rows, _, _ := buildPaginatedRows(buttons, 1, "prompts_page", map[string]any{"category_id": 77})
	if len(rows) != 5 {
		t.Fatalf("expected 4 content rows plus the pager row, got %d rows", len(rows))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rows[4][0].Action.Payload), &payload); err != nil {
		t.Fatalf("unmarshal pager payload: %v", err)
	}
	if got := payload["type"]; got != "prompts_page" {
		t.Fatalf("expected prompts_page type, got %#v", got)
	}
	if got := int(payload["page"].(float64)); got != 2 {
		t.Fatalf("expected next page=2, got %d", got)
	}
	if got := int(payload["category_id"].(float64)); got != 77 {
		t.Fatalf("expected category_id=77, got %d", got)
	}
}

func TestMainMenuFollowsSpecLayout(t *testing.T) {
	// Раскладка задана схемой из п. 1 ТЗ построчно, и проверяем её тоже
	// построчно: fitInlineRows умеет склеивать ряды сам, и без этого теста
	// перестановка кнопок в определении экрана прошла бы незамеченной.
	// Первые четыре кнопки — по одной в ряд, новые разделы — по две.
	want := [][]string{
		{"ready_prompts"},
		{"custom_prompt"},
		{"edit_photo"},
		{"couple"},
		{"kids", "trends"},
		{"greetings", "saved_photo"},
	}

	def, ok := content.Definition("main_menu")
	if !ok {
		t.Fatal("main_menu definition not found")
	}

	raw := RenderContentKeyboard(def.Keyboard, KeyboardRenderOptions{})

	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal rendered keyboard: %v", err)
	}
	if len(keyboard.Buttons) > vkInlineMaxRows {
		t.Fatalf("в меню %d строк, ВК разрешает %d", len(keyboard.Buttons), vkInlineMaxRows)
	}
	if len(keyboard.Buttons) != len(want) {
		t.Fatalf("в меню %d строк, схема ТЗ требует %d", len(keyboard.Buttons), len(want))
	}

	for row, wantRow := range want {
		gotRow := keyboard.Buttons[row]
		if len(gotRow) != len(wantRow) {
			t.Fatalf("в строке %d %d кнопок, ожидалось %d", row+1, len(gotRow), len(wantRow))
		}
		for pos, action := range wantRow {
			if got := gotRow[pos].Action.Payload; got != cbPayload(action) {
				t.Fatalf("строка %d, позиция %d: %q вместо %q", row+1, pos+1, got, cbPayload(action))
			}
		}
	}

	// Три новых раздела ТЗ требует выделить цветом среди остальных кнопок.
	highlighted := map[string]bool{
		cbPayload("kids"): true, cbPayload("trends"): true, cbPayload("greetings"): true,
	}
	for _, row := range keyboard.Buttons {
		for _, btn := range row {
			if highlighted[btn.Action.Payload] && btn.Color != "positive" {
				t.Fatalf("кнопка %q цвета %q: новые разделы выделяются positive", btn.Action.Label, btn.Color)
			}
		}
	}
}

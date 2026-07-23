package content

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestKeyboardJSONRoundTrip(t *testing.T) {
	def, ok := Definition("main_menu")
	if !ok {
		t.Fatal("main_menu definition not found")
	}

	payload, err := json.Marshal(def.Keyboard)
	if err != nil {
		t.Fatalf("marshal keyboard: %v", err)
	}

	var decoded Keyboard
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal keyboard: %v", err)
	}

	if !reflect.DeepEqual(def.Keyboard, decoded) {
		t.Fatalf("roundtrip mismatch:\nwant %#v\ngot  %#v", def.Keyboard, decoded)
	}
}

func TestBuildLegacyKeyboardUsesLegacyLabels(t *testing.T) {
	keyboard := BuildLegacyKeyboard("welcome", []LegacyButton{
		{Label: "New CTA", Payload: "free_gen"},
		{Label: "New referral", Payload: "referral"},
	})

	labels := make(map[string]string, len(keyboard.Items))
	for _, item := range keyboard.Items {
		labels[item.ActionKey] = item.Label
	}

	if got := labels["free_gen"]; got != "New CTA" {
		t.Fatalf("expected free_gen label to be replaced, got %q", got)
	}
	if got := labels["referral"]; got != "New referral" {
		t.Fatalf("expected referral label to be replaced, got %q", got)
	}
}

func TestMergeEditableKeyboardRejectsImmutableChanges(t *testing.T) {
	def, ok := Definition("welcome")
	if !ok {
		t.Fatal("welcome definition not found")
	}

	input := def.Keyboard
	input.Items[0].ActionKey = "hacked"

	if _, err := MergeEditableKeyboard("welcome", input); err == nil {
		t.Fatal("expected immutable field validation error")
	}
}

func TestMergeEditableKeyboardAllowsEditableFields(t *testing.T) {
	def, ok := Definition("saved_photo_filled")
	if !ok {
		t.Fatal("saved_photo_filled definition not found")
	}

	input := def.Keyboard
	input.Items[0].Label = "Custom label"
	input.Items[0].Row = 10
	input.Items[0].Position = 3
	input.Items[0].Visible = false
	input.Items[1].LabelOn = "Enabled label"

	merged, err := MergeEditableKeyboard("saved_photo_filled", input)
	if err != nil {
		t.Fatalf("merge editable keyboard: %v", err)
	}

	var replaceButton Button
	var toggleButton Button
	for _, item := range merged.Items {
		switch item.SlotID {
		case "saved_photo_replace":
			replaceButton = item
		case "saved_photo_toggle":
			toggleButton = item
		}
	}

	if replaceButton.Label != "Custom label" {
		t.Fatalf("expected edited label, got %q", replaceButton.Label)
	}
	if replaceButton.Row != 10 || replaceButton.Position != 3 {
		t.Fatalf("expected row/position override, got row=%d pos=%d", replaceButton.Row, replaceButton.Position)
	}
	if replaceButton.Visible {
		t.Fatal("expected visible=false to be preserved")
	}
	if toggleButton.LabelOn != "Enabled label" {
		t.Fatalf("expected label_on override, got %q", toggleButton.LabelOn)
	}
}

func TestExampleCategoryScreensHaveCTAAndBack(t *testing.T) {
	keys := []string{
		"examples_self",
		"examples_couple",
		"examples_kids",
		"examples_edit",
		"examples_greetings",
		"examples_misc",
	}

	for _, key := range keys {
		def, ok := Definition(key)
		if !ok {
			t.Fatalf("screen %q definition not found", key)
		}

		actions := make(map[string]Button, len(def.Keyboard.Items))
		for _, item := range def.Keyboard.Items {
			actions[item.ActionKey] = item
		}

		if _, ok := actions["tariffs"]; !ok {
			t.Fatalf("screen %q has no «Активировать все функции» button", key)
		}
		// Назад в категории возвращает в меню примеров, а не в общий back-сценарий.
		if _, ok := actions["examples"]; !ok {
			t.Fatalf("screen %q has no back-to-examples button", key)
		}
	}
}

func TestExamplesMenuListsAllCategories(t *testing.T) {
	def, ok := Definition("examples_collage")
	if !ok {
		t.Fatal("examples_collage definition not found")
	}

	want := map[string]bool{
		"examples_self":      false,
		"examples_couple":    false,
		"examples_kids":      false,
		"examples_edit":      false,
		"examples_greetings": false,
		"examples_misc":      false,
		"back":               false,
	}
	byAction := make(map[string]Button, len(def.Keyboard.Items))
	for _, item := range def.Keyboard.Items {
		byAction[item.ActionKey] = item
		if _, ok := want[item.ActionKey]; ok {
			want[item.ActionKey] = true
		}
	}
	for action, found := range want {
		if !found {
			t.Fatalf("examples menu has no button for action %q", action)
		}
	}

	// «Фото для себя» — первой кнопкой, «Назад» — последней строкой и одна в ряду.
	first := byAction["examples_self"]
	back := byAction["back"]
	for _, item := range def.Keyboard.Items {
		if item.ActionKey != "examples_self" && item.Row <= first.Row {
			t.Fatalf("button %q sits above or beside «Фото для себя» (rows %d vs %d)", item.ActionKey, item.Row, first.Row)
		}
		if item.ActionKey != "back" && item.Row >= back.Row {
			t.Fatalf("button %q sits below or beside «Назад» (rows %d vs %d)", item.ActionKey, item.Row, back.Row)
		}
	}
}

func TestExamplesEntryPointsAreVisible(t *testing.T) {
	for _, key := range []string{"welcome", "tariffs"} {
		def, ok := Definition(key)
		if !ok {
			t.Fatalf("screen %q definition not found", key)
		}

		var examples, back Button
		var hasExamples, hasBack bool
		for _, item := range def.Keyboard.Items {
			switch item.ActionKey {
			case "examples":
				examples, hasExamples = item, true
			case "back":
				back, hasBack = item, true
			}
		}

		if !hasExamples {
			t.Fatalf("screen %q has no «Примеры работ» button", key)
		}
		if !examples.Visible {
			t.Fatalf("screen %q has hidden «Примеры работ» button", key)
		}
		if hasBack && examples.Row >= back.Row {
			t.Fatalf("screen %q shows examples button below back button (rows %d vs %d)", key, examples.Row, back.Row)
		}
	}
}

func TestScreenMetaCoversDefaultScreens(t *testing.T) {
	for _, def := range DefaultScreens() {
		meta := ScreenMeta(def.Key)
		if meta.Title == "" {
			t.Fatalf("screen %q has empty admin title", def.Key)
		}
		if meta.SectionID == "" {
			t.Fatalf("screen %q has empty section id", def.Key)
		}
		if _, ok := ScreenSectionByID(meta.SectionID); !ok {
			t.Fatalf("screen %q points to unknown section %q", def.Key, meta.SectionID)
		}
	}
}

func TestScreenMetaFallbackUsesHumanizedTitle(t *testing.T) {
	meta := ScreenMeta("unknown_screen_key")

	if meta.SectionID != "other" {
		t.Fatalf("expected fallback section to be other, got %q", meta.SectionID)
	}
	if meta.Title != "Unknown Screen Key" {
		t.Fatalf("expected humanized fallback title, got %q", meta.Title)
	}
}

func TestActionMetaUsesHumanTitles(t *testing.T) {
	meta := ActionMeta("buy_tariff")

	if meta.SectionID != "billing" {
		t.Fatalf("expected billing section, got %q", meta.SectionID)
	}
	if meta.Title != "Покупка тарифа" {
		t.Fatalf("unexpected action title %q", meta.Title)
	}
}

func TestActionMetaFallbackUsesHumanizedTitle(t *testing.T) {
	meta := ActionMeta("very_custom_action")

	if meta.SectionID != "other" {
		t.Fatalf("expected fallback section to be other, got %q", meta.SectionID)
	}
	if meta.Title != "Very Custom Action" {
		t.Fatalf("expected humanized fallback title, got %q", meta.Title)
	}
}

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

	if got := keyboard.Items[0].Label; got != "New CTA" {
		t.Fatalf("expected first label to be replaced, got %q", got)
	}
	if got := keyboard.Items[1].Label; got != "New referral" {
		t.Fatalf("expected second label to be replaced, got %q", got)
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

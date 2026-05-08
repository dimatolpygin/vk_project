package handlers

import (
	"bytes"
	"html/template"
	"testing"
	"time"

	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

func TestBuildMessagesPageDataGroupsAndSortsScreens(t *testing.T) {
	now := time.Date(2026, time.May, 8, 12, 0, 0, 0, time.UTC)

	data := buildMessagesPageData([]*repository.Message{
		{Key: "payment_success", Text: "Payment success", Keyboard: content.Keyboard{}, UpdatedAt: now},
		{Key: "unknown_custom_screen", Text: "Unknown screen body", Keyboard: content.Keyboard{}, UpdatedAt: now},
		{Key: "main_menu", Text: "Main menu body", Keyboard: content.Keyboard{}, UpdatedAt: now},
		{Key: "welcome", Text: "Welcome body", Keyboard: content.Keyboard{}, UpdatedAt: now},
	})

	if len(data.Sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(data.Sections))
	}

	wantSections := []string{"onboarding", "navigation", "billing", "other"}
	for i, sectionID := range wantSections {
		if data.Sections[i].ID != sectionID {
			t.Fatalf("expected section %d to be %q, got %q", i, sectionID, data.Sections[i].ID)
		}
	}

	wantKeys := []string{"welcome", "main_menu", "payment_success", "unknown_custom_screen"}
	for i, key := range wantKeys {
		if data.Screens[i].Key != key {
			t.Fatalf("expected screen %d to be %q, got %q", i, key, data.Screens[i].Key)
		}
	}

	if data.DefaultSectionID != "onboarding" {
		t.Fatalf("expected default section to be onboarding, got %q", data.DefaultSectionID)
	}
}

func TestBuildMessagesPageDataUsesFallbackMetadataForUnknownScreens(t *testing.T) {
	now := time.Date(2026, time.May, 8, 12, 0, 0, 0, time.UTC)

	data := buildMessagesPageData([]*repository.Message{
		{
			Key:       "custom_runtime_screen",
			Text:      "Line one\n\nLine two with {{.Value}}",
			ImageURL:  stringPtr("https://example.com/preview.jpg"),
			Keyboard:  content.Keyboard{Items: []content.Button{{SlotID: "a"}, {SlotID: "b"}}},
			UpdatedAt: now,
		},
	})

	if len(data.Screens) != 1 {
		t.Fatalf("expected one screen, got %d", len(data.Screens))
	}

	screen := data.Screens[0]
	if screen.SectionID != "other" {
		t.Fatalf("expected fallback section other, got %q", screen.SectionID)
	}
	if screen.Title != "Custom Runtime Screen" {
		t.Fatalf("expected fallback title, got %q", screen.Title)
	}
	if !screen.HasImage {
		t.Fatal("expected has image to be true")
	}
	if !screen.HasTemplateVars {
		t.Fatal("expected template vars badge to be true")
	}
	if screen.ButtonCount != 2 {
		t.Fatalf("expected two buttons, got %d", screen.ButtonCount)
	}
	if screen.PreviewText != "Line one Line two with {{.Value}}" {
		t.Fatalf("unexpected preview text: %q", screen.PreviewText)
	}
}

func TestMessagesTemplateRendersWithPageData(t *testing.T) {
	now := time.Date(2026, time.May, 8, 12, 0, 0, 0, time.UTC)
	data := buildMessagesPageData([]*repository.Message{
		{
			Key:       "welcome",
			Text:      "Welcome body",
			Keyboard:  content.Keyboard{},
			UpdatedAt: now,
		},
	})

	tmpl := template.Must(template.New("").Funcs(tmplFuncs).ParseFiles("../../../templates/layout.html", "../../../templates/messages.html"))
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		t.Fatalf("execute messages template: %v", err)
	}
}

func stringPtr(v string) *string {
	return &v
}

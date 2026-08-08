package handlers

import (
	"html/template"
	"testing"

	"vk_neuro_bot/internal/repository"
)

func TestCategoryRequestMapsTreeFields(t *testing.T) {
	req := categoryRequest{
		Name:         "  Гендер-пати  ",
		Gender:       "any",
		SortOrder:    3,
		IsActive:     true,
		ParentID:     42,
		Section:      repository.SectionKids,
		ScreenKey:    " kids_intro ",
		MediaKind:    repository.MediaKindPhoto,
		PromptGender: "female",
	}

	in := req.toInput()

	if in.Name != "Гендер-пати" {
		t.Fatalf("expected the name to be trimmed, got %q", in.Name)
	}
	if in.ParentID == nil || *in.ParentID != 42 {
		t.Fatalf("expected parent 42, got %#v", in.ParentID)
	}
	if in.ScreenKey == nil || *in.ScreenKey != "kids_intro" {
		t.Fatalf("expected a trimmed screen key, got %#v", in.ScreenKey)
	}
	if in.PromptGender == nil || *in.PromptGender != "female" {
		t.Fatalf("expected prompt gender female, got %#v", in.PromptGender)
	}
}

func TestCategoryRequestTreatsEmptyValuesAsUnset(t *testing.T) {
	// Ноль в parent_id — это «корень раздела», а не узел с id 0; пустые строки
	// экрана и пола должны лечь в БД как NULL, иначе наследование не сработает.
	in := categoryRequest{Name: "Тренды", ParentID: 0}.toInput()

	if in.ParentID != nil {
		t.Fatalf("expected a root node, got parent %#v", in.ParentID)
	}
	if in.ScreenKey != nil {
		t.Fatalf("expected no screen key, got %#v", in.ScreenKey)
	}
	if in.PromptGender != nil {
		t.Fatalf("expected no prompt gender override, got %#v", in.PromptGender)
	}
}

func TestPromptsTemplateParses(t *testing.T) {
	// Страница разделов набита JS-шаблонными строками, и незакрытая {{...}}
	// уронила бы админку только в рантайме.
	if _, err := template.New("").Funcs(tmplFuncs).ParseFiles(
		"../../../templates/layout.html",
		"../../../templates/prompts.html",
	); err != nil {
		t.Fatalf("parse prompts template: %v", err)
	}
}

func TestPromptRequestNormalisesVideoFields(t *testing.T) {
	in := promptRequest{
		CategoryID:  81,
		Name:        "  Танец у окна  ",
		Prompt:      "cinematic portrait",
		Gender:      "female",
		MediaKind:   repository.MediaKindVideo,
		VideoPrompt: "  slow camera push in  ",
		PriceGens:   40,
		IsActive:    true,
	}.toInput()

	if in.Name != "Танец у окна" {
		t.Fatalf("название не обрезано: %q", in.Name)
	}
	if in.VideoPrompt != "slow camera push in" {
		t.Fatalf("видео-промт не обрезан: %q", in.VideoPrompt)
	}
	if in.MediaKind != repository.MediaKindVideo || in.PriceGens != 40 {
		t.Fatalf("поля видео потеряны: kind=%q price=%d", in.MediaKind, in.PriceGens)
	}
}

func TestPromptRequestKeepsPhotoPromptsAtOneGeneration(t *testing.T) {
	// Карточка фото-промта цену не показывает вовсе, поэтому в запросе её нет —
	// нулевая цена не должна означать бесплатную генерацию.
	in := promptRequest{CategoryID: 80, Name: "Обычный тренд", Prompt: "portrait"}.toInput()

	if in.MediaKind != "" && in.MediaKind != repository.MediaKindPhoto {
		t.Fatalf("неожиданный тип медиа: %q", in.MediaKind)
	}
	if in.PriceGens != 0 {
		t.Fatalf("хендлер не должен додумывать цену, это делает репозиторий: %d", in.PriceGens)
	}
}

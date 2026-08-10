package handlers

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strings"
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
		IsActive:    true,
	}.toInput()

	if in.Name != "Танец у окна" {
		t.Fatalf("название не обрезано: %q", in.Name)
	}
	if in.VideoPrompt != "slow camera push in" {
		t.Fatalf("видео-промт не обрезан: %q", in.VideoPrompt)
	}
	if in.MediaKind != repository.MediaKindVideo {
		t.Fatalf("тип медиа потерян: kind=%q", in.MediaKind)
	}
}

func TestPromptRequestCarriesNoPrice(t *testing.T) {
	// Цена видео живёт в тарифе-видеопакете. Если она снова появится в карточке
	// промта, число опять начнёт расходиться между двумя экранами админки.
	body, err := json.Marshal(promptRequest{CategoryID: 81, Name: "Тренд", MediaKind: repository.MediaKindVideo})
	if err != nil {
		t.Fatalf("marshal promptRequest: %v", err)
	}
	if strings.Contains(string(body), "price") {
		t.Fatalf("в запросе промта снова есть цена: %s", body)
	}
}

func TestTariffRequestCarriesVideoPackFlag(t *testing.T) {
	// Галочка видеопакета — единственный способ задать списание за видео,
	// поэтому она обязана доезжать до репозитория.
	in := tariffRequest{
		Name:        "  1 видео  ",
		Desc:        "  Хватает на одно видео  ",
		Price:       690,
		GensCount:   40,
		IsActive:    true,
		IsVideoPack: true,
	}.toInput()

	if in.Name != "1 видео" || in.Description != "Хватает на одно видео" {
		t.Fatalf("поля не обрезаны: name=%q desc=%q", in.Name, in.Description)
	}
	if !in.IsVideoPack || in.GensCount != 40 {
		t.Fatalf("видеопакет потерян: flag=%v gens=%d", in.IsVideoPack, in.GensCount)
	}
}

func TestVideoCostViewFallsBackWhenNoPackMarked(t *testing.T) {
	packed := []*repository.Tariff{
		{Name: "30 фотографий", GensCount: 30},
		{Name: "1 видео", GensCount: 40, IsVideoPack: true},
	}
	cost, has, hint := buildVideoCostView(packed)
	if cost != 40 || !has {
		t.Fatalf("цена видео из пакета: %d, флаг %v", cost, has)
	}
	if !strings.Contains(hint, "1 видео") {
		t.Fatalf("подсказка не называет пакет: %q", hint)
	}

	// Пакет сняли — админка обязана сказать, что цифра взята из кода, иначе
	// админ будет думать, что она настроена, и не найдёт где.
	cost, has, hint = buildVideoCostView([]*repository.Tariff{{Name: "30 фотографий", GensCount: 30}})
	if cost != repository.DefaultVideoCostGens || has {
		t.Fatalf("запасная цена: %d, флаг %v", cost, has)
	}
	if !strings.Contains(hint, "запасное значение") {
		t.Fatalf("подсказка не предупреждает о запасном значении: %q", hint)
	}
}

// Карточка промта: поля «Порядок» и «Раздел промта» были спрятаны при создании,
// и второй промт в категории заводился другими полями, чем первый. Тест держит
// разметку: оба поля есть в форме, и создание уходит в выбранный раздел.
func TestPromptFormKeepsSortAndCategoryFields(t *testing.T) {
	data := map[string]any{
		"Title":          "Категории и промты",
		"Active":         "prompts",
		"AdminBase":      "/admin",
		"CategoriesJSON": template.JS("[]"),
		"PromptsJSON":    template.JS("[]"),
	}

	tmpl := template.Must(template.New("").Funcs(tmplFuncs).ParseFiles("../../../templates/layout.html", "../../../templates/prompts.html"))

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		t.Fatalf("render prompts template: %v", err)
	}

	page := buf.String()
	for _, needle := range []string{`id="promptSort"`, `id="promptCategory"`, `id="promptCategoryHint"`} {
		if !strings.Contains(page, needle) {
			t.Fatalf("prompt form must contain %s", needle)
		}
	}
	// Скрытие полей при создании жило именно в этих двух строках.
	for _, forbidden := range []string{
		`document.getElementById('promptSortWrap').style.display = isEdit ? '' : 'none';`,
		`moveWrap.style.display = isEdit ? '' : 'none';`,
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("prompt form still hides fields on create: %s", forbidden)
		}
	}
	// Создание идёт в выбранный раздел, а не в открытый.
	if !strings.Contains(page, "categories/${targetCategoryID}/prompts") {
		t.Fatal("new prompt must be created in the selected category")
	}
}

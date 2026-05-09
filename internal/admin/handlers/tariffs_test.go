package handlers

import (
	"bytes"
	"html/template"
	"testing"

	"vk_neuro_bot/internal/repository"
)

func TestBuildTariffSummaryViewCountsAndStartingPrice(t *testing.T) {
	summary := buildTariffSummaryView([]*repository.Tariff{
		{ID: 1, Price: 799, IsActive: true},
		{ID: 2, Price: 499, IsActive: true},
		{ID: 3, Price: 299, IsActive: false},
	})

	if summary.TotalCount != 3 {
		t.Fatalf("expected total count 3, got %d", summary.TotalCount)
	}
	if summary.ActiveCount != 2 {
		t.Fatalf("expected active count 2, got %d", summary.ActiveCount)
	}
	if summary.HiddenCount != 1 {
		t.Fatalf("expected hidden count 1, got %d", summary.HiddenCount)
	}
	if summary.StartingPrice != "499 ₽" {
		t.Fatalf("unexpected starting price %q", summary.StartingPrice)
	}
}

func TestBuildTariffSummaryViewHandlesEmptyList(t *testing.T) {
	summary := buildTariffSummaryView(nil)

	if summary.TotalCount != 0 {
		t.Fatalf("expected empty total count, got %d", summary.TotalCount)
	}
	if summary.ActiveCount != 0 || summary.HiddenCount != 0 {
		t.Fatalf("unexpected counts for empty summary: %#v", summary)
	}
	if summary.StartingPrice != "—" {
		t.Fatalf("unexpected starting price %q", summary.StartingPrice)
	}
}

func TestTariffsTemplateRenders(t *testing.T) {
	data := buildTariffsPageData([]*repository.Tariff{
		{
			ID:          1,
			Name:        "Стандарт",
			Description: "20 генераций для активных пользователей",
			Price:       499,
			GensCount:   20,
			IsActive:    true,
			SortOrder:   1,
		},
	})

	tmpl := template.Must(template.New("").Funcs(tmplFuncs).ParseFiles("../../../templates/layout.html", "../../../templates/tariffs.html"))

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		t.Fatalf("render tariffs template: %v", err)
	}
}

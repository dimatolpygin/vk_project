package handlers

import (
	"bytes"
	"html/template"
	"testing"
	"time"

	"vk_neuro_bot/internal/repository"
)

func TestFillCountSeriesBuildsStableBuckets(t *testing.T) {
	now := time.Date(2026, time.May, 8, 15, 0, 0, 0, time.UTC)
	points := fillCountSeries(now, 3, []repository.DailyCount{
		{Day: time.Date(2026, time.May, 7, 0, 0, 0, 0, time.UTC), Count: 4},
	})

	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	if points[0].Label != "06.05" || points[0].Value != 0 {
		t.Fatalf("unexpected first point %#v", points[0])
	}
	if points[1].Label != "07.05" || points[1].Value != 4 {
		t.Fatalf("unexpected second point %#v", points[1])
	}
	if points[2].Label != "08.05" || points[2].Value != 0 {
		t.Fatalf("unexpected third point %#v", points[2])
	}
}

func TestBuildTopSectionViewsAggregatesCounts(t *testing.T) {
	views := buildTopSectionViews([]repository.ActionCount{
		{ActionKey: "buy_tariff", Count: 5},
		{ActionKey: "tariffs", Count: 3},
		{ActionKey: "support", Count: 2},
	})

	if len(views) < 2 {
		t.Fatalf("expected at least two sections, got %d", len(views))
	}
	if views[0].Title != "Оплата" {
		t.Fatalf("expected billing section first, got %q", views[0].Title)
	}
	if views[0].Count != 8 {
		t.Fatalf("expected aggregated billing count 8, got %d", views[0].Count)
	}
}

func TestBuildTopActionViewsUsesHumanTitles(t *testing.T) {
	views := buildTopActionViews([]repository.ActionCount{
		{ActionKey: "buy_tariff", Count: 5},
	})

	if len(views) != 1 {
		t.Fatalf("expected one top action, got %d", len(views))
	}
	if views[0].Title != "Покупка тарифа" {
		t.Fatalf("unexpected human title %q", views[0].Title)
	}
	if views[0].Section != "Оплата" {
		t.Fatalf("unexpected section %q", views[0].Section)
	}
}

func TestStatsTemplateRenders(t *testing.T) {
	now := time.Date(2026, time.May, 8, 15, 0, 0, 0, time.UTC)
	data := buildStatsPageData(
		"/admin",
		now,
		statsPeriod{Key: "30d", Days: 30, Label: "Последние 30 дней"},
		&repository.DashboardStats{
			TotalUsers:      100,
			PeriodUsers:     12,
			ActiveUsers:     40,
			TotalGens:       220,
			PeriodGens:      35,
			TotalRevenue:    8000,
			PeriodRevenue:   2400,
			TotalPaidUsers:  32,
			PeriodPayments:  8,
			TotalReferrals:  18,
			PeriodReferrals: 4,
		},
		[]repository.DailyCount{{Day: now, Count: 3}},
		[]repository.DailyCount{{Day: now, Count: 6}},
		[]repository.DailyCount{{Day: now, Count: 8}},
		[]repository.DailyAmount{{Day: now, Amount: 499}},
		[]repository.ActionCount{{ActionKey: "buy_tariff", Count: 4}},
	)

	tmpl := template.Must(template.New("").Funcs(tmplFuncs).ParseFiles("../../../templates/layout.html", "../../../templates/stats.html"))
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		t.Fatalf("render stats template: %v", err)
	}
}

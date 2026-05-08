package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

type StatsHandler struct {
	stats *repository.StatsRepo
	tmpl  *template.Template
}

type statsPageData struct {
	Title             string
	Active            string
	Period            string
	PeriodLabel       string
	PeriodDescription string
	PeriodLinks       []periodLinkView
	KPIs              []statsKPIView
	Registrations     []seriesPoint
	ActiveUsers       []seriesPoint
	Generations       []seriesPoint
	Revenue           []seriesPoint
	TopActions        []topActionView
	TopSections       []topSectionView
	ActionStateLabel  string
	SectionStateLabel string
}

type periodLinkView struct {
	Label  string
	Href   string
	Active bool
}

type statsKPIView struct {
	Title string
	Value string
	Hint  string
	Icon  string
	Tone  string
}

type seriesPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type topActionView struct {
	Title       string
	Key         string
	Description string
	Section     string
	Count       int
	Width       int
}

type topSectionView struct {
	Title       string
	Description string
	Icon        string
	Count       int
	Width       int
}

type statsPeriod struct {
	Key   string
	Days  int
	Label string
}

func NewStatsHandler(stats *repository.StatsRepo) *StatsHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/stats.html")
	return &StatsHandler{stats: stats, tmpl: tmpl}
}

func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	period := selectedStatsPeriod(r.URL.Query().Get("period"))
	now := time.Now().UTC()
	since := periodStart(now, period.Days)

	dashboard, err := h.stats.Dashboard(r.Context(), since)
	if err != nil {
		log.Error().Err(err).Msg("failed to load dashboard stats")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	registrations, err := h.stats.RegistrationSeries(r.Context(), since)
	if err != nil {
		log.Error().Err(err).Msg("failed to load registrations series")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	activeUsers, err := h.stats.ActiveUsersSeries(r.Context(), since)
	if err != nil {
		log.Error().Err(err).Msg("failed to load active users series")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	generations, err := h.stats.GenerationSeries(r.Context(), since)
	if err != nil {
		log.Error().Err(err).Msg("failed to load generations series")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	revenue, err := h.stats.RevenueSeries(r.Context(), since)
	if err != nil {
		log.Error().Err(err).Msg("failed to load revenue series")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	actionCounts, err := h.stats.ActionCounts(r.Context(), since)
	if err != nil {
		log.Error().Err(err).Msg("failed to load action counts")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := buildStatsPageData(now, period, dashboard, registrations, activeUsers, generations, revenue, actionCounts)
	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("failed to render stats template")
	}
}

func buildStatsPageData(
	now time.Time,
	period statsPeriod,
	dashboard *repository.DashboardStats,
	registrations []repository.DailyCount,
	activeUsers []repository.DailyCount,
	generations []repository.DailyCount,
	revenue []repository.DailyAmount,
	actionCounts []repository.ActionCount,
) statsPageData {
	return statsPageData{
		Title:             "Статистика",
		Active:            "stats",
		Period:            period.Key,
		PeriodLabel:       period.Label,
		PeriodDescription: "Дашборд по регистрациям, активности, генерациям, оплатам и поведению пользователей в боте.",
		PeriodLinks:       buildPeriodLinks(period),
		KPIs:              buildStatsKPIs(period, dashboard),
		Registrations:     fillCountSeries(now, period.Days, registrations),
		ActiveUsers:       fillCountSeries(now, period.Days, activeUsers),
		Generations:       fillCountSeries(now, period.Days, generations),
		Revenue:           fillAmountSeries(now, period.Days, revenue),
		TopActions:        buildTopActionViews(actionCounts),
		TopSections:       buildTopSectionViews(actionCounts),
		ActionStateLabel:  "Топ действий за выбранный период",
		SectionStateLabel: "Разделы сгруппированы по смысловым сценариям бота",
	}
}

func buildPeriodLinks(selected statsPeriod) []periodLinkView {
	periods := []statsPeriod{
		{Key: "7d", Days: 7, Label: "7 дней"},
		{Key: "30d", Days: 30, Label: "30 дней"},
		{Key: "90d", Days: 90, Label: "90 дней"},
	}

	links := make([]periodLinkView, 0, len(periods))
	for _, period := range periods {
		links = append(links, periodLinkView{
			Label:  period.Label,
			Href:   "/admin/stats?period=" + period.Key,
			Active: period.Key == selected.Key,
		})
	}
	return links
}

func buildStatsKPIs(period statsPeriod, stats *repository.DashboardStats) []statsKPIView {
	if stats == nil {
		return nil
	}
	return []statsKPIView{
		{
			Title: "Регистрации",
			Value: formatInt(stats.PeriodUsers),
			Hint:  "Всего пользователей: " + formatInt(stats.TotalUsers),
			Icon:  "bi-person-plus",
			Tone:  "primary",
		},
		{
			Title: "Активные пользователи",
			Value: formatInt(stats.ActiveUsers),
			Hint:  "За последние " + formatInt(period.Days) + " дней",
			Icon:  "bi-activity",
			Tone:  "success",
		},
		{
			Title: "Генерации",
			Value: formatInt(stats.PeriodGens),
			Hint:  "Всего генераций: " + formatInt(stats.TotalGens),
			Icon:  "bi-stars",
			Tone:  "warning",
		},
		{
			Title: "Выручка",
			Value: formatMoney(stats.PeriodRevenue),
			Hint:  "Всего выручки: " + formatMoney(stats.TotalRevenue),
			Icon:  "bi-cash-stack",
			Tone:  "info",
		},
		{
			Title: "Платящие пользователи",
			Value: formatInt(stats.TotalPaidUsers),
			Hint:  "Успешных оплат: " + formatInt(stats.PeriodPayments),
			Icon:  "bi-credit-card-2-front",
			Tone:  "accent",
		},
		{
			Title: "Рефералы",
			Value: formatInt(stats.PeriodReferrals),
			Hint:  "Всего рефералов: " + formatInt(stats.TotalReferrals),
			Icon:  "bi-gift",
			Tone:  "secondary",
		},
	}
}

func buildTopActionViews(actionCounts []repository.ActionCount) []topActionView {
	if len(actionCounts) == 0 {
		return nil
	}

	limit := 8
	if len(actionCounts) < limit {
		limit = len(actionCounts)
	}
	maxCount := actionCounts[0].Count
	views := make([]topActionView, 0, limit)
	for _, item := range actionCounts[:limit] {
		meta := content.ActionMeta(item.ActionKey)
		sectionTitle := "Прочее"
		if section, ok := content.ScreenSectionByID(meta.SectionID); ok {
			sectionTitle = section.Title
		}
		views = append(views, topActionView{
			Title:       meta.Title,
			Key:         item.ActionKey,
			Description: fallbackText(meta.Description, "Действие без дополнительного описания."),
			Section:     sectionTitle,
			Count:       item.Count,
			Width:       barWidth(item.Count, maxCount),
		})
	}
	return views
}

func buildTopSectionViews(actionCounts []repository.ActionCount) []topSectionView {
	if len(actionCounts) == 0 {
		return nil
	}

	type sectionAggregate struct {
		ID          string
		Title       string
		Description string
		Icon        string
		Order       int
		Count       int
	}

	aggregates := map[string]*sectionAggregate{}
	for _, item := range actionCounts {
		actionMeta := content.ActionMeta(item.ActionKey)
		section, ok := content.ScreenSectionByID(actionMeta.SectionID)
		if !ok {
			section = content.ScreenSection{
				ID:          "other",
				Title:       "Прочее",
				Description: "Действия без явной категории.",
				Icon:        "bi-three-dots",
				Order:       999,
			}
		}
		entry, ok := aggregates[section.ID]
		if !ok {
			entry = &sectionAggregate{
				ID:          section.ID,
				Title:       section.Title,
				Description: section.Description,
				Icon:        section.Icon,
				Order:       section.Order,
			}
			aggregates[section.ID] = entry
		}
		entry.Count += item.Count
	}

	sections := make([]sectionAggregate, 0, len(aggregates))
	maxCount := 0
	for _, item := range aggregates {
		if item.Count > maxCount {
			maxCount = item.Count
		}
		sections = append(sections, *item)
	}

	sort.Slice(sections, func(i, j int) bool {
		if sections[i].Count != sections[j].Count {
			return sections[i].Count > sections[j].Count
		}
		if sections[i].Order != sections[j].Order {
			return sections[i].Order < sections[j].Order
		}
		return sections[i].Title < sections[j].Title
	})

	limit := 6
	if len(sections) < limit {
		limit = len(sections)
	}
	views := make([]topSectionView, 0, limit)
	for _, item := range sections[:limit] {
		views = append(views, topSectionView{
			Title:       item.Title,
			Description: item.Description,
			Icon:        item.Icon,
			Count:       item.Count,
			Width:       barWidth(item.Count, maxCount),
		})
	}
	return views
}

func fillCountSeries(now time.Time, days int, rows []repository.DailyCount) []seriesPoint {
	values := make(map[string]float64, len(rows))
	for _, row := range rows {
		values[dayKey(row.Day)] = float64(row.Count)
	}
	return fillSeries(now, days, values)
}

func fillAmountSeries(now time.Time, days int, rows []repository.DailyAmount) []seriesPoint {
	values := make(map[string]float64, len(rows))
	for _, row := range rows {
		values[dayKey(row.Day)] = row.Amount
	}
	return fillSeries(now, days, values)
}

func fillSeries(now time.Time, days int, values map[string]float64) []seriesPoint {
	start := periodStart(now, days)
	points := make([]seriesPoint, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		key := dayKey(day)
		points = append(points, seriesPoint{
			Label: day.Format("02.01"),
			Value: values[key],
		})
	}
	return points
}

func selectedStatsPeriod(raw string) statsPeriod {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "7d":
		return statsPeriod{Key: "7d", Days: 7, Label: "Последние 7 дней"}
	case "90d":
		return statsPeriod{Key: "90d", Days: 90, Label: "Последние 90 дней"}
	default:
		return statsPeriod{Key: "30d", Days: 30, Label: "Последние 30 дней"}
	}
}

func periodStart(now time.Time, days int) time.Time {
	base := now.UTC().Truncate(24 * time.Hour)
	return base.AddDate(0, 0, -(days - 1))
}

func dayKey(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

func barWidth(count, maxCount int) int {
	if count <= 0 || maxCount <= 0 {
		return 0
	}
	width := int(float64(count) / float64(maxCount) * 100)
	if width < 12 {
		return 12
	}
	if width > 100 {
		return 100
	}
	return width
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}

func formatMoney(value float64) string {
	return fmt.Sprintf("%.0f ₽", value)
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

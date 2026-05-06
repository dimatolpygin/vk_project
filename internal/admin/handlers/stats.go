package handlers

import (
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

type StatsHandler struct {
	stats *repository.StatsRepo
	tmpl  *template.Template
}

func NewStatsHandler(stats *repository.StatsRepo) *StatsHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/stats.html")
	return &StatsHandler{stats: stats, tmpl: tmpl}
}

func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	s, err := h.stats.Get(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения статистики")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	topButtons, _ := h.stats.TopButtons(r.Context(), 10)

	data := map[string]any{
		"Title":      "Статистика",
		"Active":     "stats",
		"Stats":      s,
		"TopButtons": topButtons,
	}

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("ошибка рендеринга шаблона stats")
	}
}

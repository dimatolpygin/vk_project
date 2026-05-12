package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

type tariffsPageData struct {
	Title        string
	Active       string
	AdminBase    string
	Summary      tariffSummaryView
	Tariffs      []tariffItemView
	VisibleCount int
}

type tariffSummaryView struct {
	TotalCount        int
	ActiveCount       int
	HiddenCount       int
	StartingPrice     string
	StartingPriceHint string
}

type tariffItemView struct {
	ID             int
	Name           string
	Description    string
	HasDescription bool
	PriceLabel     string
	PriceInput     string
	GensCount      int
	SortOrder      int
	IsActive       bool
	StatusLabel    string
	StatusClass    string
	StatusHint     string
}

type TariffsHandler struct {
	tariffs *repository.TariffRepo
	tmpl    *template.Template
}

func NewTariffsHandler(tariffs *repository.TariffRepo) *TariffsHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/tariffs.html")
	return &TariffsHandler{tariffs: tariffs, tmpl: tmpl}
}

func (h *TariffsHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.tariffs.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения тарифов")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := buildTariffsPageData(list, GetAdminBase(r))

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("ошибка рендеринга шаблона tariffs")
	}
}

func (h *TariffsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		Desc      string  `json:"description"`
		Price     float64 `json:"price"`
		GensCount int     `json:"gens_count"`
		SortOrder int     `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	t, err := h.tariffs.Create(r.Context(), req.Name, req.Desc, req.Price, req.GensCount, req.SortOrder)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания тарифа")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(t)
}

func (h *TariffsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name      string  `json:"name"`
		Desc      string  `json:"description"`
		Price     float64 `json:"price"`
		GensCount int     `json:"gens_count"`
		SortOrder int     `json:"sort_order"`
		IsActive  bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.tariffs.Update(r.Context(), id, req.Name, req.Desc, req.Price, req.GensCount, req.SortOrder, req.IsActive); err != nil {
		log.Error().Err(err).Msg("ошибка обновления тарифа")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *TariffsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	if err := h.tariffs.Delete(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("ошибка удаления тарифа")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func buildTariffsPageData(tariffs []*repository.Tariff, base string) tariffsPageData {
	if tariffs == nil {
		tariffs = []*repository.Tariff{}
	}

	return tariffsPageData{
		Title:        "Тарифы",
		Active:       "tariffs",
		AdminBase:    base,
		Summary:      buildTariffSummaryView(tariffs),
		Tariffs:      buildTariffItemViews(tariffs),
		VisibleCount: len(tariffs),
	}
}

func buildTariffSummaryView(tariffs []*repository.Tariff) tariffSummaryView {
	summary := tariffSummaryView{
		TotalCount:        len(tariffs),
		StartingPrice:     "—",
		StartingPriceHint: "Активируйте тариф, чтобы показать стартовую цену.",
	}

	var minActive float64
	hasActive := false
	for _, tariff := range tariffs {
		if tariff == nil {
			continue
		}
		if tariff.IsActive {
			summary.ActiveCount++
			if !hasActive || tariff.Price < minActive {
				minActive = tariff.Price
				hasActive = true
			}
			continue
		}
		summary.HiddenCount++
	}

	if hasActive {
		summary.StartingPrice = formatTariffPrice(minActive)
		summary.StartingPriceHint = "Минимальная цена среди активных предложений."
	}

	return summary
}

func buildTariffItemViews(tariffs []*repository.Tariff) []tariffItemView {
	items := make([]tariffItemView, 0, len(tariffs))
	for _, tariff := range tariffs {
		if tariff == nil {
			continue
		}

		statusLabel := "Скрыт"
		statusClass := "muted"
		statusHint := "Не показывается пользователям."
		if tariff.IsActive {
			statusLabel = "Активен"
			statusClass = "active"
			statusHint = "Доступен в сценариях оплаты."
		}

		items = append(items, tariffItemView{
			ID:             tariff.ID,
			Name:           tariff.Name,
			Description:    tariff.Description,
			HasDescription: tariff.Description != "",
			PriceLabel:     formatTariffPrice(tariff.Price),
			PriceInput:     strconv.FormatFloat(tariff.Price, 'f', -1, 64),
			GensCount:      tariff.GensCount,
			SortOrder:      tariff.SortOrder,
			IsActive:       tariff.IsActive,
			StatusLabel:    statusLabel,
			StatusClass:    statusClass,
			StatusHint:     statusHint,
		})
	}
	return items
}

func formatTariffPrice(price float64) string {
	return fmt.Sprintf("%.0f ₽", price)
}

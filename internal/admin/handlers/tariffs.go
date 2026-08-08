package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

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
	// VideoCost — сколько генераций списывается за одно видео. Это объём
	// тарифа-видеопакета: страница тарифов и есть источник этой цены.
	VideoCost     int
	HasVideoPack  bool
	VideoCostHint string
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
	IsVideoPack    bool
	StatusLabel    string
	StatusClass    string
	StatusHint     string
}

// tariffRequest — тело запроса на создание и правку тарифа. is_video_pack
// помечает пакет ровно на одно видео: его gens_count и есть цена видео-генерации.
type tariffRequest struct {
	Name        string  `json:"name"`
	Desc        string  `json:"description"`
	Price       float64 `json:"price"`
	GensCount   int     `json:"gens_count"`
	SortOrder   int     `json:"sort_order"`
	IsActive    bool    `json:"is_active"`
	IsVideoPack bool    `json:"is_video_pack"`
}

func (req tariffRequest) toInput() repository.TariffInput {
	return repository.TariffInput{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Desc),
		Price:       req.Price,
		GensCount:   req.GensCount,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
		IsVideoPack: req.IsVideoPack,
	}
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
	var req tariffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	t, err := h.tariffs.Create(r.Context(), req.toInput())
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

	var req tariffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.tariffs.Update(r.Context(), id, req.toInput()); err != nil {
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

	cost, hasPack, hint := buildVideoCostView(tariffs)

	return tariffsPageData{
		Title:         "Тарифы",
		Active:        "tariffs",
		AdminBase:     base,
		Summary:       buildTariffSummaryView(tariffs),
		Tariffs:       buildTariffItemViews(tariffs),
		VisibleCount:  len(tariffs),
		VideoCost:     cost,
		HasVideoPack:  hasPack,
		VideoCostHint: hint,
	}
}

// buildVideoCostView — сколько списывается за видео и откуда взято это число.
// Пакета нет — честно говорим, что работает запасное значение из кода, иначе
// админ увидит цифру и решит, что она где-то настроена.
func buildVideoCostView(tariffs []*repository.Tariff) (cost int, hasPack bool, hint string) {
	for _, tariff := range tariffs {
		if tariff != nil && tariff.IsVideoPack {
			return tariff.GensCount, true, "Списывается за одно видео. Меняется объёмом пакета «" + tariff.Name + "»."
		}
	}
	return repository.DefaultVideoCostGens, false,
		"Ни один тариф не отмечен как пакет на 1 видео — работает запасное значение из кода. " +
			"Отметьте видеопакет, чтобы управлять ценой отсюда."
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
			IsVideoPack:    tariff.IsVideoPack,
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

package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

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

	data := map[string]any{
		"Title":   "Тарифы",
		"Active":  "tariffs",
		"Tariffs": list,
	}

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

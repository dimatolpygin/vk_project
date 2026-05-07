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

type CategoriesHandler struct {
	cats    *repository.CategoryRepo
	prompts *repository.PromptRepo
	tmpl    *template.Template
}

func NewCategoriesHandler(cats *repository.CategoryRepo, prompts *repository.PromptRepo) *CategoriesHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/prompts.html")
	return &CategoriesHandler{cats: cats, prompts: prompts, tmpl: tmpl}
}

func (h *CategoriesHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.cats.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения категорий")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cats == nil {
		cats = []*repository.Category{}
	}

	allPrompts, err := h.prompts.List(r.Context())
	if err != nil {
		allPrompts = []*repository.Prompt{}
	}
	if allPrompts == nil {
		allPrompts = []*repository.Prompt{}
	}

	type categoryView struct {
		ID         int     `json:"id"`
		Name       string  `json:"name"`
		PreviewURL *string `json:"preview_url"`
		Gender     string  `json:"gender"`
		SortOrder  int     `json:"sort_order"`
		IsActive   bool    `json:"is_active"`
	}

	catViews := make([]categoryView, 0, len(cats))
	for _, cat := range cats {
		catViews = append(catViews, categoryView{
			ID:         cat.ID,
			Name:       cat.Name,
			PreviewURL: cat.PreviewURL,
			Gender:     cat.Gender,
			SortOrder:  cat.SortOrder,
			IsActive:   cat.IsActive,
		})
	}

	catsJSON, err := json.Marshal(catViews)
	if err != nil {
		log.Warn().Err(err).Msg("не удалось сериализовать категории для admin UI")
		catsJSON = []byte("[]")
	}

	promptsJSON, err := json.Marshal(allPrompts)
	if err != nil {
		log.Warn().Err(err).Msg("не удалось сериализовать промты для admin UI")
		promptsJSON = []byte("[]")
	}

	data := map[string]any{
		"Title":          "Категории и промты",
		"Active":         "prompts",
		"Categories":     cats,
		"Prompts":        allPrompts,
		"CategoriesJSON": template.JS(string(catsJSON)),
		"PromptsJSON":    template.JS(string(promptsJSON)),
	}

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("ошибка рендеринга шаблона prompts")
	}
}

func (h *CategoriesHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Gender    string `json:"gender"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Gender == "" {
		req.Gender = "any"
	}

	cat, err := h.cats.Create(r.Context(), req.Name, req.Gender, req.SortOrder)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания категории")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cat)
}

func (h *CategoriesHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name       string  `json:"name"`
		Gender     string  `json:"gender"`
		SortOrder  int     `json:"sort_order"`
		IsActive   bool    `json:"is_active"`
		PreviewURL *string `json:"preview_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.cats.Update(r.Context(), id, req.Name, req.Gender, req.SortOrder, req.IsActive, req.PreviewURL); err != nil {
		log.Error().Err(err).Msg("ошибка обновления категории")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *CategoriesHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	if err := h.cats.Delete(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("ошибка удаления категории")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *CategoriesHandler) ListPrompts(w http.ResponseWriter, r *http.Request) {
	catID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	prompts, err := h.prompts.ListByCategoryAll(r.Context(), catID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(prompts)
}

func (h *CategoriesHandler) CreatePrompt(w http.ResponseWriter, r *http.Request) {
	catID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name      string `json:"name"`
		Prompt    string `json:"prompt"`
		Gender    string `json:"gender"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Gender == "" {
		req.Gender = "any"
	}

	p, err := h.prompts.Create(r.Context(), catID, req.Name, req.Prompt, req.Gender, req.SortOrder)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания промта")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (h *CategoriesHandler) UpdatePrompt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var req struct {
		CategoryID int    `json:"category_id"`
		Name       string `json:"name"`
		Prompt     string `json:"prompt"`
		Gender     string `json:"gender"`
		SortOrder  int    `json:"sort_order"`
		IsActive   bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.prompts.Update(r.Context(), id, req.CategoryID, req.Name, req.Prompt, req.Gender, req.SortOrder, req.IsActive); err != nil {
		log.Error().Err(err).Msg("ошибка обновления промта")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *CategoriesHandler) DeletePrompt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	if err := h.prompts.Delete(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("ошибка удаления промта")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

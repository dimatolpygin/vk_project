package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

type CategoriesHandler struct {
	cats     *repository.CategoryRepo
	prompts  *repository.PromptRepo
	messages *repository.MessageRepo
	tmpl     *template.Template
}

func NewCategoriesHandler(cats *repository.CategoryRepo, prompts *repository.PromptRepo, messages *repository.MessageRepo) *CategoriesHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/prompts.html")
	return &CategoriesHandler{cats: cats, prompts: prompts, messages: messages, tmpl: tmpl}
}

// screenDonor — с какого экрана списывается стартовый текст нового экрана узла.
// Копия, а не пустая заготовка: экран без текста ВК отбивает ошибкой, а без
// кнопок пользователь теряет «Назад».
func screenDonor(cat *repository.Category, step string) string {
	switch step {
	case "prompts":
		return "prompts_list"
	case "photo":
		return "photo_requirements"
	default:
		if cat.ScreenKey != nil && *cat.ScreenKey != "" {
			return *cat.ScreenKey
		}
		return flows.SectionRootScreen(cat.Section)
	}
}

// CreateNodeScreen заводит экран узла и возвращает его ключ. Ключ придумывает
// сервер: ручной ввод означал бы опечатку, которая уезжает пользователю голым
// текстом ключа.
func (h *CategoriesHandler) CreateNodeScreen(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	step := chi.URLParam(r, "step")
	if !content.NodeScreenStepKnown(step) {
		http.Error(w, "unknown step", http.StatusBadRequest)
		return
	}

	cat, err := h.cats.GetByID(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Int("category_id", id).Msg("ошибка получения раздела для экрана узла")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cat == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	key := content.NodeScreenKey(id, step)
	created, err := h.messages.CreateFrom(r.Context(), key, screenDonor(cat, step))
	if err != nil {
		log.Error().Err(err).Str("screen_key", key).Msg("ошибка создания экрана узла")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"key": key, "created": created})
}

// categoryRequest — тело запроса на создание и правку узла дерева.
// ParentID приходит нулём, когда узел кладут в корень раздела.
type categoryRequest struct {
	Name       string  `json:"name"`
	Gender     string  `json:"gender"`
	SortOrder  int     `json:"sort_order"`
	IsActive   bool    `json:"is_active"`
	PreviewURL *string `json:"preview_url"`
	ParentID   int     `json:"parent_id"`
	Section    string  `json:"section"`
	ScreenKey  string  `json:"screen_key"`
	// Экраны отдельных шагов узла: список промтов и запрос фото.
	PromptsScreenKey string `json:"prompts_screen_key"`
	PhotoScreenKey   string `json:"photo_screen_key"`
	MediaKind        string `json:"media_kind"`
	PromptGender     string `json:"prompt_gender"`
}

func (req categoryRequest) toInput() repository.CategoryInput {
	in := repository.CategoryInput{
		Name:       strings.TrimSpace(req.Name),
		Gender:     req.Gender,
		SortOrder:  req.SortOrder,
		IsActive:   req.IsActive,
		PreviewURL: req.PreviewURL,
		Section:    req.Section,
		MediaKind:  req.MediaKind,
	}
	if req.ParentID > 0 {
		parentID := req.ParentID
		in.ParentID = &parentID
	}
	for _, pair := range []struct {
		raw    string
		target **string
	}{
		{req.ScreenKey, &in.ScreenKey},
		{req.PromptsScreenKey, &in.PromptsScreenKey},
		{req.PhotoScreenKey, &in.PhotoScreenKey},
	} {
		if key := strings.TrimSpace(pair.raw); key != "" {
			value := key
			*pair.target = &value
		}
	}
	if promptGender := strings.TrimSpace(req.PromptGender); promptGender != "" {
		in.PromptGender = &promptGender
	}
	return in
}

// promptRequest — тело запроса на создание и правку карточки промта.
// media_kind = video включает вторую модель в цепочке, video_prompt уходит ей.
// Цены здесь нет: за видео списывается объём тарифа-видеопакета.
type promptRequest struct {
	CategoryID  int    `json:"category_id"`
	Name        string `json:"name"`
	Prompt      string `json:"prompt"`
	Gender      string `json:"gender"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
	MediaKind   string `json:"media_kind"`
	VideoPrompt string `json:"video_prompt"`
}

func (req promptRequest) toInput() repository.PromptInput {
	return repository.PromptInput{
		CategoryID:  req.CategoryID,
		Name:        strings.TrimSpace(req.Name),
		Prompt:      req.Prompt,
		Gender:      req.Gender,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
		MediaKind:   req.MediaKind,
		VideoPrompt: strings.TrimSpace(req.VideoPrompt),
	}
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
		ID               int     `json:"id"`
		Name             string  `json:"name"`
		PreviewURL       *string `json:"preview_url"`
		Gender           string  `json:"gender"`
		SortOrder        int     `json:"sort_order"`
		IsActive         bool    `json:"is_active"`
		ParentID         *int    `json:"parent_id"`
		Section          string  `json:"section"`
		ScreenKey        *string `json:"screen_key"`
		PromptsScreenKey *string `json:"prompts_screen_key"`
		PhotoScreenKey   *string `json:"photo_screen_key"`
		MediaKind        string  `json:"media_kind"`
		PromptGender     *string `json:"prompt_gender"`
		// Depth считается на сервере: дерево уже приходит в порядке обхода,
		// и UI остаётся простым списком с отступами.
		Depth int `json:"depth"`
	}

	depths := make(map[int]int, len(cats))
	catViews := make([]categoryView, 0, len(cats))
	for _, cat := range cats {
		depth := 0
		if cat.ParentID != nil {
			depth = depths[*cat.ParentID] + 1
		}
		depths[cat.ID] = depth

		catViews = append(catViews, categoryView{
			ID:               cat.ID,
			Name:             cat.Name,
			PreviewURL:       cat.PreviewURL,
			Gender:           cat.Gender,
			SortOrder:        cat.SortOrder,
			IsActive:         cat.IsActive,
			ParentID:         cat.ParentID,
			Section:          cat.Section,
			ScreenKey:        cat.ScreenKey,
			PromptsScreenKey: cat.PromptsScreenKey,
			PhotoScreenKey:   cat.PhotoScreenKey,
			MediaKind:        cat.MediaKind,
			PromptGender:     cat.PromptGender,
			Depth:            depth,
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
		"AdminBase":      GetAdminBase(r),
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
	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	in := req.toInput()
	in.IsActive = true

	cat, err := h.cats.Create(r.Context(), in)
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

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.cats.Update(r.Context(), id, req.toInput()); err != nil {
		if errors.Is(err, repository.ErrCategoryCycle) {
			http.Error(w, "нельзя вложить раздел сам в себя", http.StatusBadRequest)
			return
		}
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

	var req promptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	in := req.toInput()
	in.CategoryID = catID
	in.IsActive = true

	p, err := h.prompts.Create(r.Context(), in)
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

	var req promptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.prompts.Update(r.Context(), id, req.toInput()); err != nil {
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

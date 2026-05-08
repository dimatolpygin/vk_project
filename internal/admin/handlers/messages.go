package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

type MessagesHandler struct {
	msgs *repository.MessageRepo
	tmpl *template.Template
}

type messageSectionView struct {
	ID          string
	Title       string
	Description string
	Icon        string
	Count       int
	Order       int
}

type messageScreenView struct {
	Key             string
	Title           string
	Description     string
	SectionID       string
	SectionOrder    int
	ScreenOrder     int
	SearchText      string
	Text            string
	PreviewText     string
	ImageURL        *string
	Keyboard        content.Keyboard
	HasImage        bool
	ButtonCount     int
	HasTemplateVars bool
	UpdatedAt       string
}

type messagesPageData struct {
	Title                 string
	Active                string
	Sections              []messageSectionView
	Screens               []messageScreenView
	TotalScreens          int
	DefaultSectionID      string
	DefaultSectionTitle   string
	DefaultSectionSummary string
}

func NewMessagesHandler(msgs *repository.MessageRepo) *MessagesHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/messages.html")
	return &MessagesHandler{msgs: msgs, tmpl: tmpl}
}

func (h *MessagesHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.msgs.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения сообщений")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := buildMessagesPageData(list)
	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("ошибка рендеринга шаблона messages")
	}
}

func (h *MessagesHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	msg, err := h.msgs.Get(r.Context(), key)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msg)
}

func (h *MessagesHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key      string           `json:"key"`
		Text     string           `json:"text"`
		ImageURL *string          `json:"image_url"`
		Keyboard content.Keyboard `json:"keyboard"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	keyboard, err := content.MergeEditableKeyboard(req.Key, req.Keyboard)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.msgs.Upsert(r.Context(), req.Key, req.Text, req.ImageURL, keyboard); err != nil {
		log.Error().Err(err).Msg("ошибка сохранения сообщения")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func buildMessagesPageData(list []*repository.Message) messagesPageData {
	sectionCatalog := make(map[string]content.ScreenSection, len(content.ScreenSections()))
	for _, section := range content.ScreenSections() {
		sectionCatalog[section.ID] = section
	}

	screens := make([]messageScreenView, 0, len(list))
	sectionCounts := make(map[string]int, len(sectionCatalog))
	for _, msg := range list {
		view := buildMessageScreenView(msg, sectionCatalog)
		screens = append(screens, view)
		sectionCounts[view.SectionID]++
	}

	sort.Slice(screens, func(i, j int) bool {
		if screens[i].SectionOrder != screens[j].SectionOrder {
			return screens[i].SectionOrder < screens[j].SectionOrder
		}
		if screens[i].ScreenOrder != screens[j].ScreenOrder {
			return screens[i].ScreenOrder < screens[j].ScreenOrder
		}
		return screens[i].Key < screens[j].Key
	})

	sections := make([]messageSectionView, 0, len(sectionCatalog))
	for _, section := range content.ScreenSections() {
		count := sectionCounts[section.ID]
		if count == 0 {
			continue
		}
		sections = append(sections, messageSectionView{
			ID:          section.ID,
			Title:       section.Title,
			Description: section.Description,
			Icon:        section.Icon,
			Count:       count,
			Order:       section.Order,
		})
	}

	defaultSectionID := "other"
	defaultSectionTitle := "Сообщения"
	defaultSectionSummary := "Выберите раздел слева, чтобы отредактировать нужный экран."
	if len(sections) > 0 {
		defaultSectionID = sections[0].ID
		defaultSectionTitle = sections[0].Title
		defaultSectionSummary = sections[0].Description
	}

	return messagesPageData{
		Title:                 "Сообщения",
		Active:                "messages",
		Sections:              sections,
		Screens:               screens,
		TotalScreens:          len(screens),
		DefaultSectionID:      defaultSectionID,
		DefaultSectionTitle:   defaultSectionTitle,
		DefaultSectionSummary: defaultSectionSummary,
	}
}

func buildMessageScreenView(msg *repository.Message, sectionCatalog map[string]content.ScreenSection) messageScreenView {
	meta := content.ScreenMeta(msg.Key)
	section, ok := sectionCatalog[meta.SectionID]
	if !ok {
		section, _ = content.ScreenSectionByID("other")
		meta.SectionID = section.ID
	}

	preview := compactPreview(msg.Text)
	searchParts := []string{
		meta.Title,
		meta.Description,
		msg.Key,
		preview,
		msg.Text,
		strings.Join(meta.Keywords, " "),
	}

	return messageScreenView{
		Key:             msg.Key,
		Title:           meta.Title,
		Description:     meta.Description,
		SectionID:       section.ID,
		SectionOrder:    section.Order,
		ScreenOrder:     meta.Order,
		SearchText:      strings.ToLower(strings.Join(searchParts, " ")),
		Text:            msg.Text,
		PreviewText:     preview,
		ImageURL:        msg.ImageURL,
		Keyboard:        msg.Keyboard,
		HasImage:        msg.ImageURL != nil && strings.TrimSpace(*msg.ImageURL) != "",
		ButtonCount:     len(msg.Keyboard.Items),
		HasTemplateVars: strings.Contains(msg.Text, "{{"),
		UpdatedAt:       msg.UpdatedAt.Format("02.01.2006 15:04"),
	}
}

func compactPreview(text string) string {
	preview := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if preview == "" {
		return "Текст пока пустой."
	}

	runes := []rune(preview)
	if len(runes) <= 140 {
		return preview
	}
	return string(runes[:140]) + "…"
}

package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

type MessagesHandler struct {
	msgs *repository.MessageRepo
	tmpl *template.Template
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

	data := map[string]any{
		"Title":    "Сообщения",
		"Active":   "messages",
		"Messages": list,
	}

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

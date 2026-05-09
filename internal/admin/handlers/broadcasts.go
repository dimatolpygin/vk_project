package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/worker"
)

const broadcastsPageSize = 20
const broadcastCreateFormMaxMemory = 10 << 20

const (
	broadcastImageUploadStateNone      = "none"
	broadcastImageUploadStateUploading = "uploading"
	broadcastImageUploadStateReady     = "ready"
	broadcastImageUploadStateFailed    = "failed"
)

type broadcastStore interface {
	List(ctx context.Context, limit int) ([]*repository.Broadcast, error)
	CreateWithSnapshot(ctx context.Context, params repository.CreateBroadcastParams) (*repository.Broadcast, error)
	Get(ctx context.Context, id int64) (*repository.Broadcast, error)
}

type broadcastTaskClient interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type BroadcastsHandler struct {
	repo        broadcastStore
	asynqClient broadcastTaskClient
	tmpl        *template.Template
}

type broadcastsPageData struct {
	Title      string
	Active     string
	Broadcasts []broadcastItemView
}

type broadcastItemView struct {
	ID              int64
	AudienceLabel   string
	Status          string
	StatusLabel     string
	StatusClass     string
	TextPreview     string
	HasImage        bool
	TotalRecipients int
	SentCount       int
	FailedCount     int
	RemainingCount  int
	ProgressPercent int
	LastError       string
	CreatedAt       string
	StartedAt       string
	CompletedAt     string
	LiveUpdates     bool
}

type broadcastStatusResponse struct {
	ID              int64  `json:"id"`
	Status          string `json:"status"`
	StatusLabel     string `json:"status_label"`
	StatusClass     string `json:"status_class"`
	TotalRecipients int    `json:"total_recipients"`
	SentCount       int    `json:"sent_count"`
	FailedCount     int    `json:"failed_count"`
	RemainingCount  int    `json:"remaining_count"`
	ProgressPercent int    `json:"progress_percent"`
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at"`
	LastError       string `json:"last_error,omitempty"`
	HasImage        bool   `json:"has_image"`
}

func NewBroadcastsHandler(repo broadcastStore, asynqClient broadcastTaskClient) *BroadcastsHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/broadcasts.html")
	return &BroadcastsHandler{
		repo:        repo,
		asynqClient: asynqClient,
		tmpl:        tmpl,
	}
}

func (h *BroadcastsHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), broadcastsPageSize)
	if err != nil {
		log.Error().Err(err).Msg("failed to load broadcasts list")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := buildBroadcastsPageData(list)
	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("failed to render broadcasts template")
	}
}

func (h *BroadcastsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := parseBroadcastCreateRequest(r); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Не удалось прочитать форму рассылки.")
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		writeJSONError(w, http.StatusBadRequest, "Текст рассылки обязателен.")
		return
	}

	audience, err := repository.NormalizeBroadcastAudience(r.FormValue("audience_filter"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Неизвестная аудитория рассылки.")
		return
	}

	var imageURL *string
	if rawURL := strings.TrimSpace(r.FormValue("image_url")); rawURL != "" {
		imageURL = &rawURL
	}

	imageUploadState := normalizeBroadcastImageUploadState(r.FormValue("image_upload_state"))
	if errMessage := validateBroadcastImageUploadState(imageUploadState, imageURL); errMessage != "" {
		writeJSONError(w, http.StatusBadRequest, errMessage)
		return
	}

	broadcast, err := h.repo.CreateWithSnapshot(r.Context(), repository.CreateBroadcastParams{
		AudienceFilter: audience,
		Text:           text,
		ImageURL:       imageURL,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create broadcast")
		writeJSONError(w, http.StatusInternalServerError, "Не удалось создать рассылку.")
		return
	}

	if broadcast.TotalRecipients > 0 {
		taskPayload := worker.BroadcastPayload{BroadcastID: broadcast.ID, BatchSize: 50}
		payloadBytes, err := taskPayload.Bytes()
		if err != nil {
			log.Error().Err(err).Int64("broadcast_id", broadcast.ID).Msg("failed to encode broadcast payload")
			writeJSONError(w, http.StatusInternalServerError, "Не удалось поставить рассылку в очередь.")
			return
		}

		task := asynq.NewTask(
			worker.TaskBroadcastProcess,
			payloadBytes,
			asynq.MaxRetry(5),
			asynq.Timeout(2*time.Minute),
		)
		if _, err := h.asynqClient.Enqueue(task); err != nil {
			log.Error().Err(err).Int64("broadcast_id", broadcast.ID).Msg("failed to enqueue broadcast task")
			writeJSONError(w, http.StatusInternalServerError, "Не удалось поставить рассылку в очередь.")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"id":               broadcast.ID,
		"total_recipients": broadcast.TotalRecipients,
	})
}

func (h *BroadcastsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	broadcast, err := h.repo.Get(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Int64("broadcast_id", id).Msg("failed to load broadcast")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if broadcast == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, buildBroadcastStatusResponse(broadcast))
}

func buildBroadcastsPageData(list []*repository.Broadcast) broadcastsPageData {
	views := make([]broadcastItemView, 0, len(list))
	for _, item := range list {
		if item == nil {
			continue
		}
		views = append(views, buildBroadcastItemView(item))
	}

	return broadcastsPageData{
		Title:      "Рассылки",
		Active:     "broadcasts",
		Broadcasts: views,
	}
}

func buildBroadcastItemView(item *repository.Broadcast) broadcastItemView {
	if item == nil {
		return broadcastItemView{}
	}

	statusLabel, statusClass := broadcastStatusBadge(item.Status)

	return broadcastItemView{
		ID:              item.ID,
		AudienceLabel:   broadcastAudienceLabel(item.AudienceFilter),
		Status:          item.Status,
		StatusLabel:     statusLabel,
		StatusClass:     statusClass,
		TextPreview:     compactPreview(item.Text),
		HasImage:        item.ImageURL != nil && strings.TrimSpace(*item.ImageURL) != "",
		TotalRecipients: item.TotalRecipients,
		SentCount:       item.SentCount,
		FailedCount:     item.FailedCount,
		RemainingCount:  max(item.TotalRecipients-item.SentCount-item.FailedCount, 0),
		ProgressPercent: broadcastProgressPercent(item.TotalRecipients, item.SentCount, item.FailedCount),
		LastError:       stringValue(item.LastError),
		CreatedAt:       formatDateTime(item.CreatedAt),
		StartedAt:       formatOptionalDateTime(item.StartedAt),
		CompletedAt:     formatOptionalDateTime(item.CompletedAt),
		LiveUpdates:     item.Status == repository.BroadcastStatusQueued || item.Status == repository.BroadcastStatusProcessing,
	}
}

func buildBroadcastStatusResponse(item *repository.Broadcast) broadcastStatusResponse {
	statusLabel, statusClass := broadcastStatusBadge(item.Status)
	return broadcastStatusResponse{
		ID:              item.ID,
		Status:          item.Status,
		StatusLabel:     statusLabel,
		StatusClass:     statusClass,
		TotalRecipients: item.TotalRecipients,
		SentCount:       item.SentCount,
		FailedCount:     item.FailedCount,
		RemainingCount:  max(item.TotalRecipients-item.SentCount-item.FailedCount, 0),
		ProgressPercent: broadcastProgressPercent(item.TotalRecipients, item.SentCount, item.FailedCount),
		StartedAt:       formatOptionalDateTime(item.StartedAt),
		CompletedAt:     formatOptionalDateTime(item.CompletedAt),
		LastError:       stringValue(item.LastError),
		HasImage:        item.ImageURL != nil && strings.TrimSpace(*item.ImageURL) != "",
	}
}

func broadcastAudienceLabel(audience string) string {
	switch audience {
	case repository.BroadcastAudiencePaid:
		return "Только paid"
	case repository.BroadcastAudienceFree:
		return "Только free"
	default:
		return "Вся база"
	}
}

func broadcastStatusBadge(status string) (string, string) {
	switch status {
	case repository.BroadcastStatusCompleted:
		return "Завершена", "success"
	case repository.BroadcastStatusCompletedWithErrors:
		return "Завершена с ошибками", "warning"
	case repository.BroadcastStatusProcessing:
		return "В процессе", "primary"
	default:
		return "В очереди", "secondary"
	}
}

func broadcastProgressPercent(total, sent, failed int) int {
	if total <= 0 {
		return 100
	}
	progress := int(math.Round(float64(sent+failed) * 100 / float64(total)))
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func formatOptionalDateTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "—"
	}
	return formatDateTime(*value)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{
		"status": "error",
		"error":  message,
	})
}

func parseBroadcastCreateRequest(r *http.Request) error {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return r.ParseMultipartForm(broadcastCreateFormMaxMemory)
	}

	return r.ParseForm()
}

func normalizeBroadcastImageUploadState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", broadcastImageUploadStateNone:
		return broadcastImageUploadStateNone
	case broadcastImageUploadStateUploading:
		return broadcastImageUploadStateUploading
	case broadcastImageUploadStateReady:
		return broadcastImageUploadStateReady
	case broadcastImageUploadStateFailed:
		return broadcastImageUploadStateFailed
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func validateBroadcastImageUploadState(state string, imageURL *string) string {
	switch state {
	case broadcastImageUploadStateNone:
		return ""
	case broadcastImageUploadStateUploading:
		return "Дождитесь завершения загрузки фото перед запуском рассылки."
	case broadcastImageUploadStateFailed:
		return "Фото не удалось загрузить. Уберите вложение или повторите загрузку."
	case broadcastImageUploadStateReady:
		if imageURL == nil || strings.TrimSpace(*imageURL) == "" {
			return "Фото ещё не готово. Дождитесь загрузки или уберите вложение."
		}
		return ""
	default:
		return "Неизвестное состояние загрузки фото."
	}
}

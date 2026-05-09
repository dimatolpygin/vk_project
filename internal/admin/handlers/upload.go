package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type s3Uploader interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
	PublicURL(key string) string
}

type UploadHandler struct {
	storage s3Uploader
}

func NewUploadHandler(storage s3Uploader) *UploadHandler {
	return &UploadHandler{storage: storage}
}

func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()

	if h.storage == nil {
		http.Error(w, "S3 не настроен", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "ошибка парсинга формы", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "файл не найден", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "ошибка чтения файла", http.StatusInternalServerError)
		return
	}
	if len(data) == 0 {
		http.Error(w, "файл пустой", http.StatusBadRequest)
		return
	}

	ct := http.DetectContentType(data)
	if ct == "application/octet-stream" {
		if headerCT := header.Header.Get("Content-Type"); headerCT != "" {
			ct = headerCT
		}
	}
	if ct == "" || !strings.HasPrefix(strings.ToLower(ct), "image/") {
		http.Error(w, "доступны только image-файлы", http.StatusBadRequest)
		return
	}

	key := fmt.Sprintf("admin_uploads/%d_%s", time.Now().Unix(), header.Filename)
	uploadStartedAt := time.Now()
	if _, err := h.storage.Upload(r.Context(), key, data, ct); err != nil {
		log.Error().
			Err(err).
			Str("filename", header.Filename).
			Int("size_bytes", len(data)).
			Str("content_type", ct).
			Dur("upload_elapsed", time.Since(uploadStartedAt)).
			Dur("request_elapsed", time.Since(startedAt)).
			Msg("admin upload to storage failed")
		http.Error(w, "ошибка загрузки в S3", http.StatusInternalServerError)
		return
	}

	url := h.storage.PublicURL(key)
	logEvent := log.Info()
	if uploadElapsed := time.Since(uploadStartedAt); uploadElapsed > 5*time.Second {
		logEvent = log.Warn()
	}
	logEvent.
		Str("filename", header.Filename).
		Int("size_bytes", len(data)).
		Str("content_type", ct).
		Dur("upload_elapsed", time.Since(uploadStartedAt)).
		Dur("request_elapsed", time.Since(startedAt)).
		Msg("admin upload stored in storage")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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

	ct := header.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}

	key := fmt.Sprintf("admin_uploads/%d_%s", time.Now().Unix(), header.Filename)
	if _, err := h.storage.Upload(r.Context(), key, data, ct); err != nil {
		http.Error(w, "ошибка загрузки в S3", http.StatusInternalServerError)
		return
	}

	url := h.storage.PublicURL(key)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
}

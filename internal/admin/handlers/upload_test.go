package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeUploadStorage struct {
	uploadedType string
}

func (f *fakeUploadStorage) Upload(_ context.Context, _ string, _ []byte, contentType string) (string, error) {
	f.uploadedType = contentType
	return "ok", nil
}

func (f *fakeUploadStorage) PublicURL(key string) string {
	return "https://example.com/" + key
}

func TestUploadHandlerRejectsNonImage(t *testing.T) {
	storage := &fakeUploadStorage{}
	handler := NewUploadHandler(storage)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "note.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("plain text")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	handler.Upload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUploadHandlerAcceptsImage(t *testing.T) {
	storage := &fakeUploadStorage{}
	handler := NewUploadHandler(storage)

	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(pngBytes); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	handler.Upload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if storage.uploadedType != "image/png" {
		t.Fatalf("unexpected content type: %q", storage.uploadedType)
	}
}

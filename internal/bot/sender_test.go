package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk_neuro_bot/internal/vkgroup"
)

func TestUploadPhotoFromURLRetriesWhenVKReturnsEmptyPhoto(t *testing.T) {
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

	var (
		serverURL      string
		uploadAttempts int
		saveCalls      int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/source.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		case "/method/photos.getMessagesUploadServer":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"upload_url": serverURL + "/upload",
				},
			})
		case "/upload":
			uploadAttempts++
			file, header, err := r.FormFile("photo")
			if err != nil {
				t.Fatalf("form file: %v", err)
			}
			defer file.Close()

			if got := header.Header.Get("Content-Type"); got != "image/png" {
				t.Fatalf("expected image/png upload, got %q", got)
			}
			body, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("read upload body: %v", err)
			}
			if string(body) != string(pngBytes) {
				t.Fatalf("unexpected uploaded bytes: %v", body)
			}

			w.Header().Set("Content-Type", "application/json")
			if uploadAttempts == 1 {
				_, _ = w.Write([]byte(`{"server":1,"photo":"","hash":"retry-hash"}`))
				return
			}
			_, _ = w.Write([]byte(`{"server":1,"photo":"photo-token","hash":"retry-hash"}`))
		case "/method/photos.saveMessagesPhoto":
			saveCalls++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": []map[string]any{
					{"id": 1, "owner_id": 2},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	vk := vkgroup.New("token", 1)
	vk.SetAPIBase(server.URL + "/method")
	vk.SetHTTPClient(server.Client())

	sender := NewSender(vk, nil, nil, nil, nil)
	sender.http = server.Client()

	attachment, err := sender.uploadPhotoFromURL(context.Background(), 101, server.URL+"/source.png")
	if err != nil {
		t.Fatalf("uploadPhotoFromURL: %v", err)
	}
	if attachment != "photo2_1" {
		t.Fatalf("unexpected attachment: %q", attachment)
	}
	if uploadAttempts != 2 {
		t.Fatalf("expected 2 upload attempts, got %d", uploadAttempts)
	}
	if saveCalls != 1 {
		t.Fatalf("expected 1 save call, got %d", saveCalls)
	}
}

func TestUploadPhotoFromURLRejectsNonImageSource(t *testing.T) {
	var (
		serverURL   string
		uploadCalls int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/broken":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html>temporarily unavailable</html>"))
		case "/method/photos.getMessagesUploadServer":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"upload_url": serverURL + "/upload",
				},
			})
		case "/upload":
			uploadCalls++
			t.Fatal("upload endpoint should not be called for non-image source")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	vk := vkgroup.New("token", 1)
	vk.SetAPIBase(server.URL + "/method")
	vk.SetHTTPClient(server.Client())

	sender := NewSender(vk, nil, nil, nil, nil)
	sender.http = server.Client()

	_, err := sender.uploadPhotoFromURL(context.Background(), 101, server.URL+"/broken")
	if err == nil {
		t.Fatal("expected non-image source to fail")
	}
	if !strings.Contains(err.Error(), "did not return an image") {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadCalls != 0 {
		t.Fatalf("expected no upload calls, got %d", uploadCalls)
	}
}

func TestBuildVKUploadFilenameUsesDetectedExtension(t *testing.T) {
	got := buildVKUploadFilename("https://example.com/result", "image/png")
	if got != "result.png" {
		t.Fatalf("expected result.png, got %q", got)
	}

	got = buildVKUploadFilename("https://example.com/result.jpg?x=1", "image/png")
	if got != "result.png" {
		t.Fatalf("expected result.png after content-type normalization, got %q", got)
	}
}

package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/content"
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

// Видео уходит кадром-превью и кнопкой: настоящее видео-вложение групповому
// токену недоступно, а mp4-документ ВК показывает без обложки. Проверяем, что
// вложением идёт фото, файла в сообщении нет, а ссылка на ролик — в кнопке.
func TestSendVideoResultAttachesSceneFrameAndDownloadButton(t *testing.T) {
	const videoURL = "https://storage.example.com/video/6683.mp4"
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}

	var (
		serverURL      string
		sentAttachment string
		sentKeyboard   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/scene.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		case "/method/photos.getMessagesUploadServer":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{"upload_url": serverURL + "/upload"},
			})
		case "/upload":
			_, _ = w.Write([]byte(`{"server":1,"photo":"photo-token","hash":"h"}`))
		case "/method/photos.saveMessagesPhoto":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": []map[string]any{{"id": 77, "owner_id": -238989543}},
			})
		case "/method/messages.send":
			_ = r.ParseForm()
			sentAttachment = r.FormValue("attachment")
			sentKeyboard = r.FormValue("keyboard")
			_ = json.NewEncoder(w).Encode(map[string]any{"response": 1})
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

	def, ok := content.Definition("after_gen_video")
	if !ok {
		t.Fatal("экран after_gen_video не найден в определениях")
	}
	// Ровно три кнопки: скачать видео, ещё тренды, главное меню.
	if len(def.Keyboard.Items) != 3 {
		t.Fatalf("на экране %d кнопок, ожидалось 3", len(def.Keyboard.Items))
	}

	attachment, err := sender.uploadPhotoFromURL(context.Background(), 42, server.URL+"/scene.png")
	if err != nil {
		t.Fatalf("кадр не загрузился: %v", err)
	}

	err = sender.SendScreen(context.Background(), 42, &flows.ScreenMessage{
		Key:         "after_gen_video",
		Text:        "🎬 Готово!",
		Attachments: []string{attachment},
		Keyboard: flows.RenderContentKeyboard(def.Keyboard, flows.KeyboardRenderOptions{
			Links: map[string]string{"download_video": videoURL},
		}),
	})
	if err != nil {
		t.Fatalf("сообщение не отправилось: %v", err)
	}

	if !strings.HasPrefix(sentAttachment, "photo") {
		t.Fatalf("вложением ушло не фото: %q", sentAttachment)
	}
	if strings.Contains(sentAttachment, "doc") {
		t.Fatalf("в сообщение попал файл, хотя его быть не должно: %q", sentAttachment)
	}
	if !strings.Contains(sentKeyboard, videoURL) {
		t.Fatalf("в клавиатуре нет ссылки на видео: %s", sentKeyboard)
	}
}

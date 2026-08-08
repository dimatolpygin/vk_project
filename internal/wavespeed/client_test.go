package wavespeed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitSendsAllImages(t *testing.T) {
	var gotImages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", auth)
		}

		var body struct {
			Images []string `json:"images"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		gotImages = body.Images

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"task-123"}}`))
	}))
	defer server.Close()

	oldEndpoint, hadEndpoint := modelEndpoints["test/multi-image"]
	modelEndpoints["test/multi-image"] = server.URL
	defer func() {
		if hadEndpoint {
			modelEndpoints["test/multi-image"] = oldEndpoint
		} else {
			delete(modelEndpoints, "test/multi-image")
		}
	}()

	client := New("test-key")
	taskID, err := client.Submit(context.Background(), SubmitRequest{
		Model:  "test/multi-image",
		Prompt: "portrait",
		Images: []string{
			" https://example.com/1.png ",
			"https://example.com/2.png",
			"https://example.com/3.png",
		},
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if taskID != "task-123" {
		t.Fatalf("unexpected task id: %q", taskID)
	}

	want := []string{
		"https://example.com/1.png",
		"https://example.com/2.png",
		"https://example.com/3.png",
	}
	if len(gotImages) != len(want) {
		t.Fatalf("expected %d images, got %d: %#v", len(want), len(gotImages), gotImages)
	}
	for i := range want {
		if gotImages[i] != want[i] {
			t.Fatalf("image %d: expected %q, got %q", i, want[i], gotImages[i])
		}
	}
}

func TestSubmitRejectsEmptyImages(t *testing.T) {
	client := New("test-key")
	_, err := client.Submit(context.Background(), SubmitRequest{
		Model:  "google/nano-banana-pro",
		Prompt: "portrait",
		Images: []string{" ", ""},
	})
	if err == nil {
		t.Fatal("expected empty images error")
	}
}

// Параметры видео заданы заказчиком и пользователю не предлагаются: 9:16, 720p,
// 10 секунд. Тест держит их на месте — модель по умолчанию делает 5 секунд
// и подстраивает формат под картинку, то есть молчаливый регресс тут возможен.
func TestSubmitVideoSendsFixedParameters(t *testing.T) {
	var body struct {
		Image         string `json:"image"`
		Prompt        string `json:"prompt"`
		AspectRatio   string `json:"aspect_ratio"`
		Resolution    string `json:"resolution"`
		Duration      int    `json:"duration"`
		GenerateAudio bool   `json:"generate_audio"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"video-1"}}`))
	}))
	defer server.Close()

	oldEndpoint := modelEndpoints[VideoModelSeedance]
	modelEndpoints[VideoModelSeedance] = server.URL
	defer func() { modelEndpoints[VideoModelSeedance] = oldEndpoint }()

	client := New("test-key")
	taskID, err := client.SubmitVideo(context.Background(), SubmitVideoRequest{
		Image:         "https://example.com/scene.png",
		Prompt:        "slow camera push in",
		GenerateAudio: true,
	})
	if err != nil {
		t.Fatalf("SubmitVideo returned error: %v", err)
	}
	if taskID != "video-1" {
		t.Fatalf("unexpected task id: %q", taskID)
	}
	if body.AspectRatio != VideoAspectRatio || body.Resolution != VideoResolution || body.Duration != VideoDuration {
		t.Fatalf("параметры видео уехали: %s / %s / %d", body.AspectRatio, body.Resolution, body.Duration)
	}
	if body.Image != "https://example.com/scene.png" || body.Prompt != "slow camera push in" {
		t.Fatalf("тело запроса собрано неверно: %+v", body)
	}
}

func TestSubmitVideoRejectsEmptyScene(t *testing.T) {
	// Пустая картинка означала бы, что первое звено цепочки отработало вхолостую,
	// а мы всё равно платим за видео.
	if _, err := New("test-key").SubmitVideo(context.Background(), SubmitVideoRequest{Prompt: "dance"}); err == nil {
		t.Fatal("ожидалась ошибка на пустом изображении")
	}
	if _, err := New("test-key").SubmitVideo(context.Background(), SubmitVideoRequest{Image: "https://example.com/a.png"}); err == nil {
		t.Fatal("ожидалась ошибка на пустом промте")
	}
}

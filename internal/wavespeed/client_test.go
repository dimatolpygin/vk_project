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

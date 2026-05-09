package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"vk_neuro_bot/internal/repository"
)

type fakeBroadcastStore struct {
	list      []*repository.Broadcast
	createFn  func(context.Context, repository.CreateBroadcastParams) (*repository.Broadcast, error)
	getByIDFn func(context.Context, int64) (*repository.Broadcast, error)
}

func (f *fakeBroadcastStore) List(ctx context.Context, limit int) ([]*repository.Broadcast, error) {
	return f.list, nil
}

func (f *fakeBroadcastStore) CreateWithSnapshot(ctx context.Context, params repository.CreateBroadcastParams) (*repository.Broadcast, error) {
	return f.createFn(ctx, params)
}

func (f *fakeBroadcastStore) Get(ctx context.Context, id int64) (*repository.Broadcast, error) {
	return f.getByIDFn(ctx, id)
}

type fakeBroadcastTaskClient struct {
	tasks []*asynq.Task
}

func (f *fakeBroadcastTaskClient) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{}, nil
}

func TestBroadcastsHandlerCreateValidation(t *testing.T) {
	store := &fakeBroadcastStore{
		createFn: func(context.Context, repository.CreateBroadcastParams) (*repository.Broadcast, error) {
			t.Fatal("create should not be called for invalid payload")
			return nil, nil
		},
	}
	handler := &BroadcastsHandler{repo: store, asynqClient: &fakeBroadcastTaskClient{}}

	form := url.Values{}
	form.Set("audience_filter", repository.BroadcastAudienceAll)
	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Текст рассылки обязателен") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestBroadcastsHandlerCreateEnqueuesTask(t *testing.T) {
	queue := &fakeBroadcastTaskClient{}
	store := &fakeBroadcastStore{
		createFn: func(_ context.Context, params repository.CreateBroadcastParams) (*repository.Broadcast, error) {
			if params.AudienceFilter != repository.BroadcastAudiencePaid {
				t.Fatalf("unexpected audience: %q", params.AudienceFilter)
			}
			if params.Text != "hello" {
				t.Fatalf("unexpected text: %q", params.Text)
			}
			if params.ImageURL == nil || *params.ImageURL != "https://example.com/image.png" {
				t.Fatalf("unexpected image url: %#v", params.ImageURL)
			}
			return &repository.Broadcast{
				ID:              42,
				TotalRecipients: 3,
			}, nil
		},
	}
	handler := &BroadcastsHandler{repo: store, asynqClient: queue}

	form := url.Values{}
	form.Set("audience_filter", repository.BroadcastAudiencePaid)
	form.Set("text", "hello")
	form.Set("image_url", "https://example.com/image.png")
	form.Set("image_upload_state", broadcastImageUploadStateReady)

	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(queue.tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(queue.tasks))
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["id"].(float64) != 42 {
		t.Fatalf("unexpected response id: %#v", payload["id"])
	}
}

func TestBroadcastsHandlerCreateRejectsUploadingImage(t *testing.T) {
	store := &fakeBroadcastStore{
		createFn: func(context.Context, repository.CreateBroadcastParams) (*repository.Broadcast, error) {
			t.Fatal("create should not be called while image upload is in progress")
			return nil, nil
		},
	}
	handler := &BroadcastsHandler{repo: store, asynqClient: &fakeBroadcastTaskClient{}}

	form := url.Values{}
	form.Set("audience_filter", repository.BroadcastAudienceAll)
	form.Set("text", "hello")
	form.Set("image_upload_state", broadcastImageUploadStateUploading)

	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Дождитесь завершения загрузки фото") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestBroadcastsHandlerCreateRejectsImageWithoutUploadedURL(t *testing.T) {
	store := &fakeBroadcastStore{
		createFn: func(context.Context, repository.CreateBroadcastParams) (*repository.Broadcast, error) {
			t.Fatal("create should not be called without uploaded image URL")
			return nil, nil
		},
	}
	handler := &BroadcastsHandler{repo: store, asynqClient: &fakeBroadcastTaskClient{}}

	form := url.Values{}
	form.Set("audience_filter", repository.BroadcastAudienceAll)
	form.Set("text", "hello")
	form.Set("image_upload_state", broadcastImageUploadStateReady)

	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Фото ещё не готово") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestBroadcastsHandlerCreateAcceptsMultipartForm(t *testing.T) {
	queue := &fakeBroadcastTaskClient{}
	store := &fakeBroadcastStore{
		createFn: func(_ context.Context, params repository.CreateBroadcastParams) (*repository.Broadcast, error) {
			if params.AudienceFilter != repository.BroadcastAudienceAll {
				t.Fatalf("unexpected audience: %q", params.AudienceFilter)
			}
			if params.Text != "hello from multipart" {
				t.Fatalf("unexpected text: %q", params.Text)
			}
			return &repository.Broadcast{
				ID:              7,
				TotalRecipients: 1,
			}, nil
		},
	}
	handler := &BroadcastsHandler{repo: store, asynqClient: queue}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("audience_filter", repository.BroadcastAudienceAll); err != nil {
		t.Fatalf("write audience field: %v", err)
	}
	if err := writer.WriteField("text", "hello from multipart"); err != nil {
		t.Fatalf("write text field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(queue.tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(queue.tasks))
	}
}

func TestBuildBroadcastsPageDataAndTemplateRender(t *testing.T) {
	now := time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)
	completed := now.Add(2 * time.Minute)
	data := buildBroadcastsPageData([]*repository.Broadcast{
		{
			ID:              5,
			AudienceFilter:  repository.BroadcastAudienceFree,
			Text:            "Line one\n\nLine two",
			Status:          repository.BroadcastStatusProcessing,
			TotalRecipients: 10,
			SentCount:       4,
			FailedCount:     1,
			CreatedAt:       now,
			CompletedAt:     &completed,
		},
	})

	if len(data.Broadcasts) != 1 {
		t.Fatalf("expected one broadcast, got %d", len(data.Broadcasts))
	}
	if data.Broadcasts[0].AudienceLabel != "Только free" {
		t.Fatalf("unexpected audience label: %q", data.Broadcasts[0].AudienceLabel)
	}
	if data.Broadcasts[0].ProgressPercent != 50 {
		t.Fatalf("unexpected progress: %d", data.Broadcasts[0].ProgressPercent)
	}

	tmpl := template.Must(template.New("").Funcs(tmplFuncs).ParseFiles("../../../templates/layout.html", "../../../templates/broadcasts.html"))
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		t.Fatalf("execute broadcasts template: %v", err)
	}
}

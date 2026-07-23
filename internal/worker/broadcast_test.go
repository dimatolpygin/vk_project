package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"vk_neuro_bot/internal/repository"
)

type fakeWorkerBroadcastRepo struct {
	broadcast     *repository.Broadcast
	deliveries    []*repository.BroadcastDelivery
	hasPending    bool
	sentIDs       []int64
	failedByID    map[int64]string
	refreshResult *repository.Broadcast
}

func (f *fakeWorkerBroadcastRepo) Get(context.Context, int64) (*repository.Broadcast, error) {
	return f.broadcast, nil
}

func (f *fakeWorkerBroadcastRepo) ClaimPendingDeliveries(_ context.Context, _ int64, _ int, _ time.Time) ([]*repository.BroadcastDelivery, error) {
	return f.deliveries, nil
}

func (f *fakeWorkerBroadcastRepo) MarkDeliverySent(_ context.Context, deliveryID int64) error {
	f.sentIDs = append(f.sentIDs, deliveryID)
	return nil
}

func (f *fakeWorkerBroadcastRepo) MarkDeliveryFailed(_ context.Context, deliveryID int64, errText string) error {
	if f.failedByID == nil {
		f.failedByID = map[int64]string{}
	}
	f.failedByID[deliveryID] = errText
	return nil
}

func (f *fakeWorkerBroadcastRepo) RefreshStatus(context.Context, int64) (*repository.Broadcast, error) {
	return f.refreshResult, nil
}

func (f *fakeWorkerBroadcastRepo) HasPendingDeliveries(context.Context, int64, time.Time) (bool, error) {
	return f.hasPending, nil
}

type fakeWorkerBroadcastSender struct {
	failVKID map[int64]error
	calls    []int64
	ctaFlags []bool
}

func (f *fakeWorkerBroadcastSender) SendBroadcast(_ context.Context, vkID int64, _ string, _ *string, _ int64, ctaEnabled bool) error {
	f.calls = append(f.calls, vkID)
	f.ctaFlags = append(f.ctaFlags, ctaEnabled)
	if err := f.failVKID[vkID]; err != nil {
		return err
	}
	return nil
}

type fakeWorkerTaskClient struct {
	tasks []*asynq.Task
}

func (f *fakeWorkerTaskClient) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{}, nil
}

func TestBroadcastHandlerProcessTaskSuccess(t *testing.T) {
	repo := &fakeWorkerBroadcastRepo{
		broadcast: &repository.Broadcast{
			ID:     1,
			Text:   "hello",
			Status: repository.BroadcastStatusQueued,
		},
		deliveries: []*repository.BroadcastDelivery{
			{ID: 11, BroadcastID: 1, UserVKID: 1001},
		},
		hasPending: false,
		refreshResult: &repository.Broadcast{
			ID:     1,
			Status: repository.BroadcastStatusCompleted,
		},
	}
	sender := &fakeWorkerBroadcastSender{}
	queue := &fakeWorkerTaskClient{}
	handler := NewBroadcastHandler(repo, sender, queue)

	payloadBytes, _ := BroadcastPayload{BroadcastID: 1, BatchSize: 10}.Bytes()
	task := asynq.NewTask(TaskBroadcastProcess, payloadBytes)

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(repo.sentIDs) != 1 || repo.sentIDs[0] != 11 {
		t.Fatalf("unexpected sent ids: %#v", repo.sentIDs)
	}
	if len(queue.tasks) != 0 {
		t.Fatalf("expected no follow-up tasks, got %d", len(queue.tasks))
	}
}

func TestBroadcastHandlerPassesCTAFlagToSender(t *testing.T) {
	repo := &fakeWorkerBroadcastRepo{
		broadcast: &repository.Broadcast{
			ID:         3,
			Text:       "hello",
			Status:     repository.BroadcastStatusQueued,
			CTAEnabled: true,
		},
		deliveries: []*repository.BroadcastDelivery{
			{ID: 31, BroadcastID: 3, UserVKID: 3001},
		},
		refreshResult: &repository.Broadcast{ID: 3, Status: repository.BroadcastStatusCompleted},
	}
	sender := &fakeWorkerBroadcastSender{}
	handler := NewBroadcastHandler(repo, sender, &fakeWorkerTaskClient{})

	payloadBytes, _ := BroadcastPayload{BroadcastID: 3, BatchSize: 10}.Bytes()
	if err := handler.ProcessTask(context.Background(), asynq.NewTask(TaskBroadcastProcess, payloadBytes)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.ctaFlags) != 1 || !sender.ctaFlags[0] {
		t.Fatalf("expected cta flag to reach sender, got %#v", sender.ctaFlags)
	}
}

func TestBroadcastHandlerProcessTaskPartialFailureAndRequeue(t *testing.T) {
	repo := &fakeWorkerBroadcastRepo{
		broadcast: &repository.Broadcast{
			ID:     2,
			Text:   "hello",
			Status: repository.BroadcastStatusProcessing,
		},
		deliveries: []*repository.BroadcastDelivery{
			{ID: 21, BroadcastID: 2, UserVKID: 2001},
			{ID: 22, BroadcastID: 2, UserVKID: 2002},
		},
		hasPending: true,
		refreshResult: &repository.Broadcast{
			ID:     2,
			Status: repository.BroadcastStatusProcessing,
		},
	}
	sender := &fakeWorkerBroadcastSender{
		failVKID: map[int64]error{2002: errors.New("vk send failed")},
	}
	queue := &fakeWorkerTaskClient{}
	handler := NewBroadcastHandler(repo, sender, queue)

	payloadBytes, _ := BroadcastPayload{BroadcastID: 2, BatchSize: 2}.Bytes()
	task := asynq.NewTask(TaskBroadcastProcess, payloadBytes)

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(repo.sentIDs) != 1 || repo.sentIDs[0] != 21 {
		t.Fatalf("unexpected sent ids: %#v", repo.sentIDs)
	}
	if got := repo.failedByID[22]; got != "vk send failed" {
		t.Fatalf("unexpected failure map: %#v", repo.failedByID)
	}
	if len(queue.tasks) != 1 {
		t.Fatalf("expected one follow-up task, got %d", len(queue.tasks))
	}
}

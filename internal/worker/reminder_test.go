package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"vk_neuro_bot/internal/repository"
)

type fakeReminderUsers struct {
	user *repository.User
}

func (f *fakeReminderUsers) GetByVKID(context.Context, int64) (*repository.User, error) {
	return f.user, nil
}

type fakeReminderTariffs struct {
	active []*repository.Tariff
	byID   map[int]*repository.Tariff
}

func (f *fakeReminderTariffs) ListActive(context.Context) ([]*repository.Tariff, error) {
	return f.active, nil
}

func (f *fakeReminderTariffs) GetByID(_ context.Context, id int) (*repository.Tariff, error) {
	return f.byID[id], nil
}

type fakeReminderSender struct {
	sent []*repository.Tariff
}

func (f *fakeReminderSender) SendPaymentReminder(_ context.Context, _ int64, tariff *repository.Tariff) error {
	f.sent = append(f.sent, tariff)
	return nil
}

type fakeReminderActivity struct {
	lastAt   *time.Time
	recorded []repository.ActivityEvent
}

func (f *fakeReminderActivity) Record(_ context.Context, event repository.ActivityEvent) error {
	f.recorded = append(f.recorded, event)
	return nil
}

func (f *fakeReminderActivity) LastEventAt(context.Context, int64, string) (*time.Time, error) {
	return f.lastAt, nil
}

type fakeReminderConfig struct {
	values map[string]string
}

func (f *fakeReminderConfig) Get(_ context.Context, key string) (string, error) {
	return f.values[key], nil
}

func reminderTask(t *testing.T, vkID int64) *asynq.Task {
	t.Helper()
	payload, err := PaymentReminderPayload{UserVKID: vkID}.Bytes()
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}
	return asynq.NewTask(TaskPaymentReminder, payload)
}

func defaultReminderTariffs() *fakeReminderTariffs {
	cheap := &repository.Tariff{ID: 9, Name: "Мини", Price: 49, GensCount: 1, IsActive: true}
	three := &repository.Tariff{ID: 5, Name: "3 генерации", Price: 90, GensCount: 3, IsActive: true}
	starter := &repository.Tariff{ID: 1, Name: "Стартовый", Price: 199, GensCount: 10, IsActive: true}
	return &fakeReminderTariffs{
		active: []*repository.Tariff{starter, three, cheap},
		byID:   map[int]*repository.Tariff{1: starter, 5: three, 9: cheap},
	}
}

func TestPaymentReminderPicksThreeGenerationsTariff(t *testing.T) {
	sender := &fakeReminderSender{}
	activity := &fakeReminderActivity{}
	handler := NewPaymentReminderHandler(
		&fakeReminderUsers{user: &repository.User{VKID: 100, Status: "free", FreeGens: 1}},
		defaultReminderTariffs(),
		sender,
		activity,
		&fakeReminderConfig{},
	)

	if err := handler.ProcessTask(context.Background(), reminderTask(t, 100)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected one reminder, got %d", len(sender.sent))
	}
	if got := sender.sent[0].GensCount; got != 3 {
		t.Fatalf("expected tariff with 3 generations, got %d", got)
	}
	if len(activity.recorded) != 1 || activity.recorded[0].EventType != repository.ActivityEventPaymentReminderSent {
		t.Fatalf("expected reminder activity event, got %#v", activity.recorded)
	}
}

func TestPaymentReminderSkipsPaidUser(t *testing.T) {
	sender := &fakeReminderSender{}
	handler := NewPaymentReminderHandler(
		&fakeReminderUsers{user: &repository.User{VKID: 101, Status: "paid", PaidGens: 10}},
		defaultReminderTariffs(),
		sender,
		&fakeReminderActivity{},
		&fakeReminderConfig{},
	)

	if err := handler.ProcessTask(context.Background(), reminderTask(t, 101)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("expected no reminder for paid user, got %d", len(sender.sent))
	}
}

func TestPaymentReminderRespectsCooldown(t *testing.T) {
	recently := time.Now().Add(-2 * time.Hour)
	sender := &fakeReminderSender{}
	handler := NewPaymentReminderHandler(
		&fakeReminderUsers{user: &repository.User{VKID: 102, Status: "free"}},
		defaultReminderTariffs(),
		sender,
		&fakeReminderActivity{lastAt: &recently},
		&fakeReminderConfig{},
	)

	if err := handler.ProcessTask(context.Background(), reminderTask(t, 102)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("expected no reminder inside cooldown, got %d", len(sender.sent))
	}
}

func TestPaymentReminderUsesConfiguredTariff(t *testing.T) {
	sender := &fakeReminderSender{}
	handler := NewPaymentReminderHandler(
		&fakeReminderUsers{user: &repository.User{VKID: 103, Status: "free"}},
		defaultReminderTariffs(),
		sender,
		&fakeReminderActivity{},
		&fakeReminderConfig{values: map[string]string{reminderTariffConfigKey: "1"}},
	)

	if err := handler.ProcessTask(context.Background(), reminderTask(t, 103)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.sent) != 1 || sender.sent[0].ID != 1 {
		t.Fatalf("expected configured tariff to win, got %#v", sender.sent)
	}
}

func TestPaymentReminderFallsBackToCheapestTariff(t *testing.T) {
	sender := &fakeReminderSender{}
	tariffs := &fakeReminderTariffs{
		active: []*repository.Tariff{
			{ID: 1, Name: "Стартовый", Price: 199, GensCount: 10, IsActive: true},
			{ID: 9, Name: "Мини", Price: 49, GensCount: 1, IsActive: true},
		},
	}
	handler := NewPaymentReminderHandler(
		&fakeReminderUsers{user: &repository.User{VKID: 104, Status: "free"}},
		tariffs,
		sender,
		&fakeReminderActivity{},
		&fakeReminderConfig{},
	)

	if err := handler.ProcessTask(context.Background(), reminderTask(t, 104)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.sent) != 1 || sender.sent[0].ID != 9 {
		t.Fatalf("expected cheapest tariff fallback, got %#v", sender.sent)
	}
}

func TestPaymentReminderSkipsWithoutTariffs(t *testing.T) {
	sender := &fakeReminderSender{}
	handler := NewPaymentReminderHandler(
		&fakeReminderUsers{user: &repository.User{VKID: 105, Status: "free"}},
		&fakeReminderTariffs{},
		sender,
		&fakeReminderActivity{},
		&fakeReminderConfig{},
	)

	if err := handler.ProcessTask(context.Background(), reminderTask(t, 105)); err != nil {
		t.Fatalf("process task: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("expected no reminder without tariffs, got %d", len(sender.sent))
	}
}

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ActivityEventUserRegistered        = "user_registered"
	ActivityEventMessageReceived       = "message_received"
	ActivityEventButtonClick           = "button_click"
	ActivityEventGenerationStarted     = "generation_started"
	ActivityEventPaymentSucceeded      = "payment_succeeded"
	ActivityEventSettingsChanged       = "settings_changed"
	ActivityEventSavedPhotoSaved       = "saved_photo_saved"
	ActivityEventSavedPhotoToggled     = "saved_photo_toggled"
	ActivityEventSubscriptionConfirmed = "subscription_confirmed"
	ActivityEventReferralCreated       = "referral_created"
	ActivityEventReferralBonusAwarded  = "referral_bonus_awarded"
	ActivityEventPaymentReminderSent   = "payment_reminder_sent"
)

type ActivityEvent struct {
	UserVKID  int64
	EventType string
	ActionKey string
	ScreenKey string
	Meta      map[string]any
}

type ActivityRepo struct {
	db *pgxpool.Pool
}

func NewActivityRepo(db *pgxpool.Pool) *ActivityRepo {
	return &ActivityRepo{db: db}
}

func (r *ActivityRepo) Record(ctx context.Context, event ActivityEvent) error {
	meta, err := json.Marshal(event.Meta)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO activity_events (user_vk_id, event_type, action_key, screen_key, meta)
		VALUES ($1, $2, $3, $4, $5)`,
		event.UserVKID, event.EventType, nullIfEmpty(event.ActionKey), nullIfEmpty(event.ScreenKey), meta)
	return err
}

// LastEventAt возвращает время последнего события указанного типа у пользователя.
// nil означает, что события ещё не было.
func (r *ActivityRepo) LastEventAt(ctx context.Context, vkID int64, eventType string) (*time.Time, error) {
	var createdAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT created_at
		FROM activity_events
		WHERE user_vk_id = $1 AND event_type = $2
		ORDER BY created_at DESC
		LIMIT 1`, vkID, eventType).Scan(&createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &createdAt, nil
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	BroadcastAudienceAll  = "all"
	BroadcastAudienceFree = "free"
	BroadcastAudiencePaid = "paid"

	BroadcastStatusQueued              = "queued"
	BroadcastStatusProcessing          = "processing"
	BroadcastStatusCompleted           = "completed"
	BroadcastStatusCompletedWithErrors = "completed_with_errors"

	BroadcastDeliveryStatusPending    = "pending"
	BroadcastDeliveryStatusProcessing = "processing"
	BroadcastDeliveryStatusSent       = "sent"
	BroadcastDeliveryStatusFailed     = "failed"
)

var ErrInvalidBroadcastAudience = errors.New("invalid broadcast audience")

type Broadcast struct {
	ID              int64
	AudienceFilter  string
	Text            string
	ImageURL        *string
	VkAttachment    *string
	Status          string
	TotalRecipients int
	SentCount       int
	FailedCount     int
	LastError       *string
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type BroadcastDelivery struct {
	ID                  int64
	BroadcastID         int64
	UserVKID            int64
	Status              string
	Attempts            int
	ErrorText           *string
	ProcessingStartedAt *time.Time
	SentAt              *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateBroadcastParams struct {
	AudienceFilter string
	Text           string
	ImageURL       *string
}

type BroadcastRepo struct {
	db *pgxpool.Pool
}

func NewBroadcastRepo(db *pgxpool.Pool) *BroadcastRepo {
	return &BroadcastRepo{db: db}
}

func NormalizeBroadcastAudience(audience string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(audience)) {
	case BroadcastAudienceAll:
		return BroadcastAudienceAll, nil
	case BroadcastAudienceFree:
		return BroadcastAudienceFree, nil
	case BroadcastAudiencePaid:
		return BroadcastAudiencePaid, nil
	default:
		return "", ErrInvalidBroadcastAudience
	}
}

func (r *BroadcastRepo) CreateWithSnapshot(ctx context.Context, params CreateBroadcastParams) (*Broadcast, error) {
	audience, err := NormalizeBroadcastAudience(params.AudienceFilter)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	broadcast := &Broadcast{}
	if err := tx.QueryRow(ctx, `
		INSERT INTO broadcasts (audience_filter, text, image_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
		RETURNING id, audience_filter, text, image_url, vk_attachment, status,
		          total_recipients, sent_count, failed_count, last_error,
		          started_at, completed_at, created_at, updated_at`,
		audience, params.Text, params.ImageURL, BroadcastStatusQueued,
	).Scan(
		&broadcast.ID, &broadcast.AudienceFilter, &broadcast.Text, &broadcast.ImageURL,
		&broadcast.VkAttachment, &broadcast.Status, &broadcast.TotalRecipients,
		&broadcast.SentCount, &broadcast.FailedCount, &broadcast.LastError,
		&broadcast.StartedAt, &broadcast.CompletedAt, &broadcast.CreatedAt, &broadcast.UpdatedAt,
	); err != nil {
		return nil, err
	}

	filterSQL := `TRUE`
	filterArgs := []any{broadcast.ID}
	if audience != BroadcastAudienceAll {
		filterSQL = `status = $2`
		filterArgs = append(filterArgs, audience)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO broadcast_deliveries (broadcast_id, user_vk_id, status, created_at, updated_at)
		SELECT $1, vk_id, '%s', now(), now()
		FROM users
		WHERE %s`,
		BroadcastDeliveryStatusPending, filterSQL,
	)
	cmdTag, err := tx.Exec(ctx, insertSQL, filterArgs...)
	if err != nil {
		return nil, err
	}
	broadcast.TotalRecipients = int(cmdTag.RowsAffected())

	status := BroadcastStatusQueued
	var completedAt *time.Time
	if broadcast.TotalRecipients == 0 {
		status = BroadcastStatusCompleted
		now := time.Now().UTC()
		completedAt = &now
	}

	if err := tx.QueryRow(ctx, `
		UPDATE broadcasts
		SET total_recipients = $2,
		    status = $3,
		    completed_at = $4,
		    updated_at = now()
		WHERE id = $1
		RETURNING id, audience_filter, text, image_url, vk_attachment, status,
		          total_recipients, sent_count, failed_count, last_error,
		          started_at, completed_at, created_at, updated_at`,
		broadcast.ID, broadcast.TotalRecipients, status, completedAt,
	).Scan(
		&broadcast.ID, &broadcast.AudienceFilter, &broadcast.Text, &broadcast.ImageURL,
		&broadcast.VkAttachment, &broadcast.Status, &broadcast.TotalRecipients,
		&broadcast.SentCount, &broadcast.FailedCount, &broadcast.LastError,
		&broadcast.StartedAt, &broadcast.CompletedAt, &broadcast.CreatedAt, &broadcast.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return broadcast, nil
}

func (r *BroadcastRepo) List(ctx context.Context, limit int) ([]*Broadcast, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, audience_filter, text, image_url, vk_attachment, status,
		       total_recipients, sent_count, failed_count, last_error,
		       started_at, completed_at, created_at, updated_at
		FROM broadcasts
		ORDER BY created_at DESC, id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Broadcast
	for rows.Next() {
		item := &Broadcast{}
		if err := rows.Scan(
			&item.ID, &item.AudienceFilter, &item.Text, &item.ImageURL, &item.VkAttachment,
			&item.Status, &item.TotalRecipients, &item.SentCount, &item.FailedCount,
			&item.LastError, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	return list, rows.Err()
}

func (r *BroadcastRepo) Get(ctx context.Context, id int64) (*Broadcast, error) {
	item := &Broadcast{}
	err := r.db.QueryRow(ctx, `
		SELECT id, audience_filter, text, image_url, vk_attachment, status,
		       total_recipients, sent_count, failed_count, last_error,
		       started_at, completed_at, created_at, updated_at
		FROM broadcasts
		WHERE id = $1`, id).
		Scan(
			&item.ID, &item.AudienceFilter, &item.Text, &item.ImageURL, &item.VkAttachment,
			&item.Status, &item.TotalRecipients, &item.SentCount, &item.FailedCount,
			&item.LastError, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt,
		)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *BroadcastRepo) GetVKAttachment(ctx context.Context, id int64) (*string, error) {
	var attachment *string
	err := r.db.QueryRow(ctx, `SELECT vk_attachment FROM broadcasts WHERE id = $1`, id).Scan(&attachment)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return attachment, err
}

func (r *BroadcastRepo) SetVKAttachment(ctx context.Context, id int64, attachment string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE broadcasts
		SET vk_attachment = COALESCE(vk_attachment, $2),
		    updated_at = now()
		WHERE id = $1`,
		id, attachment)
	return err
}

func (r *BroadcastRepo) ClaimPendingDeliveries(ctx context.Context, broadcastID int64, batchSize int, staleBefore time.Time) ([]*BroadcastDelivery, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM broadcast_deliveries
			WHERE broadcast_id = $1
			  AND (
				status = $2
				OR (status = $3 AND processing_started_at < $4)
			  )
			ORDER BY id
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		)
		UPDATE broadcast_deliveries d
		SET status = $3,
		    attempts = d.attempts + 1,
		    error_text = NULL,
		    processing_started_at = now(),
		    updated_at = now()
		FROM candidates
		WHERE d.id = candidates.id
		RETURNING d.id, d.broadcast_id, d.user_vk_id, d.status, d.attempts,
		          d.error_text, d.processing_started_at, d.sent_at, d.created_at, d.updated_at`,
		broadcastID, BroadcastDeliveryStatusPending, BroadcastDeliveryStatusProcessing, staleBefore, batchSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*BroadcastDelivery
	for rows.Next() {
		item := &BroadcastDelivery{}
		if err := rows.Scan(
			&item.ID, &item.BroadcastID, &item.UserVKID, &item.Status, &item.Attempts,
			&item.ErrorText, &item.ProcessingStartedAt, &item.SentAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(list) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE broadcasts
			SET status = $2,
			    started_at = COALESCE(started_at, now()),
			    updated_at = now()
			WHERE id = $1
			  AND status IN ($3, $4)`,
			broadcastID, BroadcastStatusProcessing, BroadcastStatusQueued, BroadcastStatusProcessing,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *BroadcastRepo) MarkDeliverySent(ctx context.Context, deliveryID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE broadcast_deliveries
		SET status = $2,
		    error_text = NULL,
		    sent_at = now(),
		    updated_at = now()
		WHERE id = $1`,
		deliveryID, BroadcastDeliveryStatusSent,
	)
	return err
}

func (r *BroadcastRepo) MarkDeliveryFailed(ctx context.Context, deliveryID int64, errText string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE broadcast_deliveries
		SET status = $2,
		    error_text = $3,
		    sent_at = NULL,
		    updated_at = now()
		WHERE id = $1`,
		deliveryID, BroadcastDeliveryStatusFailed, truncateErrorText(errText),
	)
	return err
}

func (r *BroadcastRepo) RefreshStatus(ctx context.Context, broadcastID int64) (*Broadcast, error) {
	item := &Broadcast{}
	err := r.db.QueryRow(ctx, `
		WITH stats AS (
			SELECT
				COUNT(*) FILTER (WHERE status = $2) AS pending_count,
				COUNT(*) FILTER (WHERE status = $3) AS processing_count,
				COUNT(*) FILTER (WHERE status = $4) AS sent_count,
				COUNT(*) FILTER (WHERE status = $5) AS failed_count
			FROM broadcast_deliveries
			WHERE broadcast_id = $1
		)
		UPDATE broadcasts b
		SET sent_count = stats.sent_count,
		    failed_count = stats.failed_count,
		    status = CASE
				WHEN b.total_recipients = 0 THEN $6
				WHEN stats.pending_count = 0 AND stats.processing_count = 0 AND stats.failed_count > 0 THEN $7
				WHEN stats.pending_count = 0 AND stats.processing_count = 0 THEN $6
				WHEN stats.sent_count > 0 OR stats.failed_count > 0 OR stats.processing_count > 0 THEN $8
				ELSE b.status
			END,
		    completed_at = CASE
				WHEN b.total_recipients = 0 THEN COALESCE(b.completed_at, now())
				WHEN stats.pending_count = 0 AND stats.processing_count = 0 THEN COALESCE(b.completed_at, now())
				ELSE NULL
			END,
		    last_error = CASE
				WHEN stats.failed_count > 0 THEN (
					SELECT error_text
					FROM broadcast_deliveries
					WHERE broadcast_id = $1
					  AND status = $5
					  AND error_text IS NOT NULL
					ORDER BY updated_at DESC, id DESC
					LIMIT 1
				)
				ELSE NULL
			END,
		    updated_at = now()
		FROM stats
		WHERE b.id = $1
		RETURNING b.id, b.audience_filter, b.text, b.image_url, b.vk_attachment, b.status,
		          b.total_recipients, b.sent_count, b.failed_count, b.last_error,
		          b.started_at, b.completed_at, b.created_at, b.updated_at`,
		broadcastID,
		BroadcastDeliveryStatusPending,
		BroadcastDeliveryStatusProcessing,
		BroadcastDeliveryStatusSent,
		BroadcastDeliveryStatusFailed,
		BroadcastStatusCompleted,
		BroadcastStatusCompletedWithErrors,
		BroadcastStatusProcessing,
	).Scan(
		&item.ID, &item.AudienceFilter, &item.Text, &item.ImageURL, &item.VkAttachment,
		&item.Status, &item.TotalRecipients, &item.SentCount, &item.FailedCount,
		&item.LastError, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *BroadcastRepo) HasPendingDeliveries(ctx context.Context, broadcastID int64, staleBefore time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM broadcast_deliveries
			WHERE broadcast_id = $1
			  AND (
				status = $2
				OR (status = $3 AND processing_started_at < $4)
			  )
		)`,
		broadcastID, BroadcastDeliveryStatusPending, BroadcastDeliveryStatusProcessing, staleBefore,
	).Scan(&exists)
	return exists, err
}

func truncateErrorText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 1000 {
		return text
	}
	return text[:1000]
}

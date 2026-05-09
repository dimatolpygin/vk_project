package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoGenerationsAvailable = errors.New("no generations available")

const (
	BalanceKindFree = "free"
	BalanceKindPaid = "paid"
)

type Generation struct {
	ID                 int64
	UserVKID           int64
	Type               string
	Prompt             string
	InputPhotoURL      *string
	OutputPhotoURL     *string
	WavespeedTaskID    *string
	Model              string
	Status             string
	ErrorText          *string
	CreatedAt          time.Time
	ChargedBalanceKind string
	BalanceRefunded    bool
}

type GenerationRepo struct {
	db *pgxpool.Pool
}

func NewGenerationRepo(db *pgxpool.Pool) *GenerationRepo {
	return &GenerationRepo{db: db}
}

type generationRowScanner interface {
	Scan(dest ...any) error
}

func scanGeneration(row generationRowScanner, g *Generation) error {
	return row.Scan(
		&g.ID,
		&g.UserVKID,
		&g.Type,
		&g.Prompt,
		&g.InputPhotoURL,
		&g.OutputPhotoURL,
		&g.WavespeedTaskID,
		&g.Model,
		&g.Status,
		&g.ErrorText,
		&g.CreatedAt,
		&g.ChargedBalanceKind,
		&g.BalanceRefunded,
	)
}

func pickChargedBalanceKind(paidGens, freeGens int) (string, bool) {
	switch {
	case paidGens > 0:
		return BalanceKindPaid, true
	case freeGens > 0:
		return BalanceKindFree, true
	default:
		return "", false
	}
}

func legacyBalanceKindFromStatus(status string) string {
	if status == BalanceKindPaid {
		return BalanceKindPaid
	}
	return BalanceKindFree
}

func (r *GenerationRepo) CreateChargedGeneration(ctx context.Context, userVKID int64, genType, prompt, model string, inputURL *string) (*Generation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var freeGens, paidGens int
	if err := tx.QueryRow(ctx, `
		SELECT free_gens, paid_gens
		FROM users
		WHERE vk_id = $1
		FOR UPDATE`, userVKID).
		Scan(&freeGens, &paidGens); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNoGenerationsAvailable
		}
		return nil, err
	}

	chargedBalanceKind, ok := pickChargedBalanceKind(paidGens, freeGens)
	if !ok {
		return nil, ErrNoGenerationsAvailable
	}

	switch chargedBalanceKind {
	case BalanceKindPaid:
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET paid_gens = paid_gens - 1, updated_at = now()
			WHERE vk_id = $1`, userVKID); err != nil {
			return nil, err
		}
	case BalanceKindFree:
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET free_gens = free_gens - 1, updated_at = now()
			WHERE vk_id = $1`, userVKID); err != nil {
			return nil, err
		}
	}

	g := &Generation{}
	err = scanGeneration(tx.QueryRow(ctx, `
		INSERT INTO generations (user_vk_id, type, prompt, model, input_photo_url, charged_balance_kind)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
		          wavespeed_task_id, COALESCE(model,''), status, error_text, created_at,
		          COALESCE(charged_balance_kind, ''), balance_refunded`,
		userVKID, genType, prompt, model, inputURL, chargedBalanceKind), g)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

func (r *GenerationRepo) SetWavespeedTaskID(ctx context.Context, id int64, taskID string) error {
	_, err := r.db.Exec(ctx, `UPDATE generations SET wavespeed_task_id = $2, status = 'processing' WHERE id = $1`, id, taskID)
	return err
}

func (r *GenerationRepo) SetCompleted(ctx context.Context, id int64, outputURL string) error {
	_, err := r.db.Exec(ctx, `UPDATE generations SET status = 'completed', output_photo_url = $2 WHERE id = $1`, id, outputURL)
	return err
}

func (r *GenerationRepo) SetFailed(ctx context.Context, id int64, errText string) error {
	_, err := r.db.Exec(ctx, `UPDATE generations SET status = 'failed', error_text = $2 WHERE id = $1`, id, errText)
	return err
}

func (r *GenerationRepo) RefundGenerationCharge(ctx context.Context, generationID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		userVKID           int64
		chargedBalanceKind string
		balanceRefunded    bool
	)
	if err := tx.QueryRow(ctx, `
		SELECT user_vk_id, COALESCE(charged_balance_kind, ''), balance_refunded
		FROM generations
		WHERE id = $1
		FOR UPDATE`, generationID).
		Scan(&userVKID, &chargedBalanceKind, &balanceRefunded); err != nil {
		return err
	}

	if balanceRefunded {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return nil
	}

	if chargedBalanceKind == "" {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM users WHERE vk_id = $1 FOR UPDATE`, userVKID).Scan(&status); err != nil {
			return err
		}
		chargedBalanceKind = legacyBalanceKindFromStatus(status)
	}

	switch chargedBalanceKind {
	case BalanceKindPaid:
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET paid_gens = paid_gens + 1, updated_at = now()
			WHERE vk_id = $1`, userVKID); err != nil {
			return err
		}
	default:
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET free_gens = free_gens + 1, updated_at = now()
			WHERE vk_id = $1`, userVKID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE generations SET balance_refunded = true WHERE id = $1`, generationID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *GenerationRepo) GetByTaskID(ctx context.Context, taskID string) (*Generation, error) {
	g := &Generation{}
	err := scanGeneration(r.db.QueryRow(ctx, `
		SELECT id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
		       wavespeed_task_id, COALESCE(model,''), status, error_text, created_at,
		       COALESCE(charged_balance_kind, ''), balance_refunded
		FROM generations WHERE wavespeed_task_id = $1`, taskID), g)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (r *GenerationRepo) GetByID(ctx context.Context, id int64) (*Generation, error) {
	g := &Generation{}
	err := scanGeneration(r.db.QueryRow(ctx, `
		SELECT id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
		       wavespeed_task_id, COALESCE(model,''), status, error_text, created_at,
		       COALESCE(charged_balance_kind, ''), balance_refunded
		FROM generations WHERE id = $1`, id), g)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (r *GenerationRepo) CountByUser(ctx context.Context, vkID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM generations WHERE user_vk_id = $1`, vkID).Scan(&n)
	return n, err
}

func (r *GenerationRepo) TotalCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM generations`).Scan(&n)
	return n, err
}

func (r *GenerationRepo) TodayCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM generations WHERE created_at >= current_date`).Scan(&n)
	return n, err
}

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
	OutputVideoURL     *string
	WavespeedTaskID    *string
	Model              string
	Status             string
	ErrorText          *string
	CreatedAt          time.Time
	ChargedBalanceKind string
	BalanceRefunded    bool
	// CostGens — во сколько генераций обошлась задача, ChargedPaidGens и
	// ChargedFreeGens — как эта стоимость разложилась по двум балансам.
	CostGens        int
	ChargedPaidGens int
	ChargedFreeGens int
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
		&g.OutputVideoURL,
		&g.WavespeedTaskID,
		&g.Model,
		&g.Status,
		&g.ErrorText,
		&g.CreatedAt,
		&g.ChargedBalanceKind,
		&g.BalanceRefunded,
		&g.CostGens,
		&g.ChargedPaidGens,
		&g.ChargedFreeGens,
	)
}

const generationColumns = `id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
	          output_video_url, wavespeed_task_id, COALESCE(model,''), status, error_text, created_at,
	          COALESCE(charged_balance_kind, ''), balance_refunded,
	          COALESCE(cost_gens, 1), COALESCE(charged_paid_gens, 0), COALESCE(charged_free_gens, 0)`

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

// splitCharge раскладывает стоимость по балансам: сначала тратится платный,
// остаток добирается из бесплатного. Видео стоит десятки генераций и в один
// баланс может не поместиться, поэтому «либо-либо» здесь уже не работает.
func splitCharge(cost, paidGens, freeGens int) (paid, free int, ok bool) {
	if cost < 1 {
		cost = 1
	}
	if paidGens+freeGens < cost {
		return 0, 0, false
	}
	paid = cost
	if paid > paidGens {
		paid = paidGens
	}
	return paid, cost - paid, true
}

func legacyBalanceKindFromStatus(status string) string {
	if status == BalanceKindPaid {
		return BalanceKindPaid
	}
	return BalanceKindFree
}

// CreateChargedGeneration списывает cost генераций и заводит запись задачи.
// cost меньше единицы трактуется как одна генерация — так вызывающий код,
// написанный до этапа 10, продолжает работать без изменений.
func (r *GenerationRepo) CreateChargedGeneration(ctx context.Context, userVKID int64, genType, prompt, model string, inputURL *string, cost int) (*Generation, error) {
	if cost < 1 {
		cost = 1
	}

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

	chargedPaid, chargedFree, ok := splitCharge(cost, paidGens, freeGens)
	if !ok {
		return nil, ErrNoGenerationsAvailable
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET paid_gens = paid_gens - $2, free_gens = free_gens - $3, updated_at = now()
		WHERE vk_id = $1`, userVKID, chargedPaid, chargedFree); err != nil {
		return nil, err
	}

	// charged_balance_kind остаётся для отчётов и старого кода возврата:
	// он не умеет в раздельное списание, поэтому пишем преобладающий баланс.
	chargedBalanceKind := BalanceKindFree
	if chargedPaid > 0 {
		chargedBalanceKind = BalanceKindPaid
	}

	g := &Generation{}
	err = scanGeneration(tx.QueryRow(ctx, `
		INSERT INTO generations (user_vk_id, type, prompt, model, input_photo_url, charged_balance_kind,
		                         cost_gens, charged_paid_gens, charged_free_gens)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+generationColumns,
		userVKID, genType, prompt, model, inputURL, chargedBalanceKind,
		cost, chargedPaid, chargedFree), g)
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

// SetVideoCompleted закрывает видео-задачу: сцена и само видео сохраняются
// отдельными колонками, потому что промежуточный кадр тоже нужен для разбора
// жалоб «получилось не то».
func (r *GenerationRepo) SetVideoCompleted(ctx context.Context, id int64, sceneURL, videoURL string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE generations
		SET status = 'completed', output_photo_url = $2, output_video_url = $3
		WHERE id = $1`, id, sceneURL, videoURL)
	return err
}

// SetSceneReady фиксирует промежуточный кадр: если видео-звено упадёт, будет
// видно, что сцена собралась и проблема дальше по цепочке.
func (r *GenerationRepo) SetSceneReady(ctx context.Context, id int64, sceneURL string) error {
	_, err := r.db.Exec(ctx, `UPDATE generations SET output_photo_url = $2 WHERE id = $1`, id, sceneURL)
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
		chargedPaid        int
		chargedFree        int
	)
	if err := tx.QueryRow(ctx, `
		SELECT user_vk_id, COALESCE(charged_balance_kind, ''), balance_refunded,
		       COALESCE(charged_paid_gens, 0), COALESCE(charged_free_gens, 0)
		FROM generations
		WHERE id = $1
		FOR UPDATE`, generationID).
		Scan(&userVKID, &chargedBalanceKind, &balanceRefunded, &chargedPaid, &chargedFree); err != nil {
		return err
	}

	if balanceRefunded {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return nil
	}

	// Записи до этапа 10 раскладки списания не хранят — там всегда одна
	// генерация с одного баланса, и его ещё нужно угадать по статусу.
	if chargedPaid == 0 && chargedFree == 0 {
		if chargedBalanceKind == "" {
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM users WHERE vk_id = $1 FOR UPDATE`, userVKID).Scan(&status); err != nil {
				return err
			}
			chargedBalanceKind = legacyBalanceKindFromStatus(status)
		}
		if chargedBalanceKind == BalanceKindPaid {
			chargedPaid = 1
		} else {
			chargedFree = 1
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET paid_gens = paid_gens + $2, free_gens = free_gens + $3, updated_at = now()
		WHERE vk_id = $1`, userVKID, chargedPaid, chargedFree); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `UPDATE generations SET balance_refunded = true WHERE id = $1`, generationID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *GenerationRepo) GetByTaskID(ctx context.Context, taskID string) (*Generation, error) {
	g := &Generation{}
	err := scanGeneration(r.db.QueryRow(ctx, `
		SELECT `+generationColumns+`
		FROM generations WHERE wavespeed_task_id = $1`, taskID), g)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (r *GenerationRepo) GetByID(ctx context.Context, id int64) (*Generation, error) {
	g := &Generation{}
	err := scanGeneration(r.db.QueryRow(ctx, `
		SELECT `+generationColumns+`
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

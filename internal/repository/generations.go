package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Generation struct {
	ID              int64
	UserVKID        int64
	Type            string
	Prompt          string
	InputPhotoURL   *string
	OutputPhotoURL  *string
	WavespeedTaskID *string
	Model           string
	Status          string
	ErrorText       *string
	CreatedAt       time.Time
}

type GenerationRepo struct {
	db *pgxpool.Pool
}

func NewGenerationRepo(db *pgxpool.Pool) *GenerationRepo {
	return &GenerationRepo{db: db}
}

func (r *GenerationRepo) Create(ctx context.Context, userVKID int64, genType, prompt, model string, inputURL *string) (*Generation, error) {
	g := &Generation{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO generations (user_vk_id, type, prompt, model, input_photo_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
		          wavespeed_task_id, COALESCE(model,''), status, error_text, created_at`,
		userVKID, genType, prompt, model, inputURL).
		Scan(&g.ID, &g.UserVKID, &g.Type, &g.Prompt, &g.InputPhotoURL, &g.OutputPhotoURL,
			&g.WavespeedTaskID, &g.Model, &g.Status, &g.ErrorText, &g.CreatedAt)
	return g, err
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

func (r *GenerationRepo) GetByTaskID(ctx context.Context, taskID string) (*Generation, error) {
	g := &Generation{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
		       wavespeed_task_id, COALESCE(model,''), status, error_text, created_at
		FROM generations WHERE wavespeed_task_id = $1`, taskID).
		Scan(&g.ID, &g.UserVKID, &g.Type, &g.Prompt, &g.InputPhotoURL, &g.OutputPhotoURL,
			&g.WavespeedTaskID, &g.Model, &g.Status, &g.ErrorText, &g.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (r *GenerationRepo) GetByID(ctx context.Context, id int64) (*Generation, error) {
	g := &Generation{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_vk_id, type, COALESCE(prompt,''), input_photo_url, output_photo_url,
		       wavespeed_task_id, COALESCE(model,''), status, error_text, created_at
		FROM generations WHERE id = $1`, id).
		Scan(&g.ID, &g.UserVKID, &g.Type, &g.Prompt, &g.InputPhotoURL, &g.OutputPhotoURL,
			&g.WavespeedTaskID, &g.Model, &g.Status, &g.ErrorText, &g.CreatedAt)
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

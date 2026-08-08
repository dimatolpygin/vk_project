package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Prompt struct {
	ID         int     `json:"id"`
	CategoryID int     `json:"category_id"`
	Name       string  `json:"name"`
	Prompt     string  `json:"prompt"`
	PreviewURL *string `json:"preview_url"`
	Gender     string  `json:"gender"`
	SortOrder  int     `json:"sort_order"`
	IsActive   bool    `json:"is_active"`
	// MediaKind решает, чем заканчивается генерация: фото или видео.
	MediaKind string `json:"media_kind"`
	// VideoPrompt уходит в видео-модель вторым звеном цепочки. Первым звеном
	// работает Prompt: по нему фото-модель собирает сцену из фото пользователя.
	VideoPrompt string `json:"video_prompt"`
	// PriceGens — сколько генераций списывается за этот промт (п. 8.3 ТЗ).
	PriceGens int `json:"price_gens"`
}

// IsVideo — видео-промт: генерация пойдёт по цепочке из двух моделей.
func (p *Prompt) IsVideo() bool {
	return p != nil && p.MediaKind == MediaKindVideo
}

// Cost — стоимость промта в генерациях. Ноль в базе означает старую запись,
// заведённую до этапа 10, — такая стоит одну генерацию.
func (p *Prompt) Cost() int {
	if p == nil || p.PriceGens < 1 {
		return 1
	}
	return p.PriceGens
}

const promptColumns = `id, category_id, name, prompt, preview_url, gender, sort_order, is_active,
	       COALESCE(media_kind, 'photo'), COALESCE(video_prompt, ''), COALESCE(price_gens, 1)`

// PromptInput — поля карточки промта, которые правит админка.
type PromptInput struct {
	CategoryID  int
	Name        string
	Prompt      string
	Gender      string
	SortOrder   int
	IsActive    bool
	MediaKind   string
	VideoPrompt string
	PriceGens   int
}

func (in PromptInput) normalized() PromptInput {
	if in.Gender == "" {
		in.Gender = GenderAny
	}
	if in.MediaKind != MediaKindVideo {
		in.MediaKind = MediaKindPhoto
	}
	if in.PriceGens < 1 {
		in.PriceGens = 1
	}
	return in
}

type PromptRepo struct {
	db *pgxpool.Pool
}

func NewPromptRepo(db *pgxpool.Pool) *PromptRepo {
	return &PromptRepo{db: db}
}

func (r *PromptRepo) ListByCategory(ctx context.Context, categoryID int, gender string) ([]*Prompt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+promptColumns+`
		FROM prompts
		WHERE category_id = $1 AND is_active = true AND (gender = 'any' OR gender = $2)
		ORDER BY sort_order`, categoryID, gender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *PromptRepo) ListByCategoryAll(ctx context.Context, categoryID int) ([]*Prompt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+promptColumns+`
		FROM prompts
		WHERE category_id = $1 AND is_active = true
		ORDER BY sort_order`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *PromptRepo) List(ctx context.Context) ([]*Prompt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+promptColumns+`
		FROM prompts ORDER BY category_id, sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *PromptRepo) GetByID(ctx context.Context, id int) (*Prompt, error) {
	p := &Prompt{}
	err := scanPrompt(r.db.QueryRow(ctx, `
		SELECT `+promptColumns+`
		FROM prompts WHERE id = $1`, id), p)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PromptRepo) Create(ctx context.Context, in PromptInput) (*Prompt, error) {
	in = in.normalized()
	p := &Prompt{}
	err := scanPrompt(r.db.QueryRow(ctx, `
		INSERT INTO prompts (category_id, name, prompt, gender, sort_order, media_kind, video_prompt, price_gens)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+promptColumns,
		in.CategoryID, in.Name, in.Prompt, in.Gender, in.SortOrder, in.MediaKind, in.VideoPrompt, in.PriceGens), p)
	return p, err
}

func (r *PromptRepo) Update(ctx context.Context, id int, in PromptInput) error {
	in = in.normalized()
	_, err := r.db.Exec(ctx, `
		UPDATE prompts
		SET category_id=$2, name=$3, prompt=$4, gender=$5, sort_order=$6, is_active=$7,
		    media_kind=$8, video_prompt=$9, price_gens=$10
		WHERE id=$1`,
		id, in.CategoryID, in.Name, in.Prompt, in.Gender, in.SortOrder, in.IsActive,
		in.MediaKind, in.VideoPrompt, in.PriceGens)
	return err
}

func (r *PromptRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM prompts WHERE id = $1`, id)
	return err
}

type promptRowScanner interface {
	Scan(dest ...any) error
}

func scanPrompt(row promptRowScanner, p *Prompt) error {
	return row.Scan(&p.ID, &p.CategoryID, &p.Name, &p.Prompt, &p.PreviewURL, &p.Gender,
		&p.SortOrder, &p.IsActive, &p.MediaKind, &p.VideoPrompt, &p.PriceGens)
}

func scanPrompts(rows pgx.Rows) ([]*Prompt, error) {
	var ps []*Prompt
	for rows.Next() {
		p := &Prompt{}
		if err := scanPrompt(rows, p); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, nil
}

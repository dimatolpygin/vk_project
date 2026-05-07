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
}

type PromptRepo struct {
	db *pgxpool.Pool
}

func NewPromptRepo(db *pgxpool.Pool) *PromptRepo {
	return &PromptRepo{db: db}
}

func (r *PromptRepo) ListByCategory(ctx context.Context, categoryID int, gender string) ([]*Prompt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, category_id, name, prompt, preview_url, gender, sort_order, is_active
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
		SELECT id, category_id, name, prompt, preview_url, gender, sort_order, is_active
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
		SELECT id, category_id, name, prompt, preview_url, gender, sort_order, is_active
		FROM prompts ORDER BY category_id, sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *PromptRepo) GetByID(ctx context.Context, id int) (*Prompt, error) {
	p := &Prompt{}
	err := r.db.QueryRow(ctx, `
		SELECT id, category_id, name, prompt, preview_url, gender, sort_order, is_active
		FROM prompts WHERE id = $1`, id).
		Scan(&p.ID, &p.CategoryID, &p.Name, &p.Prompt, &p.PreviewURL, &p.Gender, &p.SortOrder, &p.IsActive)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PromptRepo) Create(ctx context.Context, categoryID int, name, prompt, gender string, sortOrder int) (*Prompt, error) {
	p := &Prompt{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO prompts (category_id, name, prompt, gender, sort_order)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, category_id, name, prompt, preview_url, gender, sort_order, is_active`,
		categoryID, name, prompt, gender, sortOrder).
		Scan(&p.ID, &p.CategoryID, &p.Name, &p.Prompt, &p.PreviewURL, &p.Gender, &p.SortOrder, &p.IsActive)
	return p, err
}

func (r *PromptRepo) Update(ctx context.Context, id, categoryID int, name, prompt, gender string, sortOrder int, active bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE prompts SET category_id=$2, name=$3, prompt=$4, gender=$5, sort_order=$6, is_active=$7
		WHERE id=$1`, id, categoryID, name, prompt, gender, sortOrder, active)
	return err
}

func (r *PromptRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM prompts WHERE id = $1`, id)
	return err
}

func scanPrompts(rows pgx.Rows) ([]*Prompt, error) {
	var ps []*Prompt
	for rows.Next() {
		p := &Prompt{}
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.Name, &p.Prompt, &p.PreviewURL, &p.Gender, &p.SortOrder, &p.IsActive); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, nil
}

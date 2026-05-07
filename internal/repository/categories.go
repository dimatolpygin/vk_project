package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Category struct {
	ID         int
	Name       string
	GroupID    *int
	PreviewURL *string
	Gender     string
	SortOrder  int
	IsActive   bool
}

type CategoryRepo struct {
	db *pgxpool.Pool
}

func NewCategoryRepo(db *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{db: db}
}

func (r *CategoryRepo) ListActive(ctx context.Context, gender string) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, group_id, preview_url, gender, sort_order, is_active
		FROM categories
		WHERE is_active = true AND (gender = 'any' OR gender = $1)
		ORDER BY sort_order`, gender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (r *CategoryRepo) ListActiveCouple(ctx context.Context) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, group_id, preview_url, gender, sort_order, is_active
		FROM categories
		WHERE is_active = true AND gender = 'couple'
		ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (r *CategoryRepo) List(ctx context.Context) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, group_id, preview_url, gender, sort_order, is_active
		FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (r *CategoryRepo) GetByID(ctx context.Context, id int) (*Category, error) {
	c := &Category{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, group_id, preview_url, gender, sort_order, is_active
		FROM categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.GroupID, &c.PreviewURL, &c.Gender, &c.SortOrder, &c.IsActive)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return c, err
}

func (r *CategoryRepo) Create(ctx context.Context, name, gender string, sortOrder int) (*Category, error) {
	c := &Category{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO categories (name, gender, sort_order) VALUES ($1, $2, $3)
		RETURNING id, name, group_id, preview_url, gender, sort_order, is_active`,
		name, gender, sortOrder).
		Scan(&c.ID, &c.Name, &c.GroupID, &c.PreviewURL, &c.Gender, &c.SortOrder, &c.IsActive)
	return c, err
}

func (r *CategoryRepo) Update(ctx context.Context, id int, name, gender string, sortOrder int, active bool, previewURL *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE categories SET name=$2, gender=$3, sort_order=$4, is_active=$5, preview_url=$6 WHERE id=$1`,
		id, name, gender, sortOrder, active, previewURL)
	return err
}

func (r *CategoryRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}

func scanCategories(rows pgx.Rows) ([]*Category, error) {
	var cats []*Category
	for rows.Next() {
		c := &Category{}
		if err := rows.Scan(&c.ID, &c.Name, &c.GroupID, &c.PreviewURL, &c.Gender, &c.SortOrder, &c.IsActive); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}

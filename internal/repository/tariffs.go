package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Tariff struct {
	ID          int
	Name        string
	Description string
	Price       float64
	GensCount   int
	IsActive    bool
	SortOrder   int
	CreatedAt   time.Time
}

type TariffRepo struct {
	db *pgxpool.Pool
}

func NewTariffRepo(db *pgxpool.Pool) *TariffRepo {
	return &TariffRepo{db: db}
}

func (r *TariffRepo) ListActive(ctx context.Context) ([]*Tariff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, COALESCE(description,''), price, gens_count, is_active, sort_order, created_at
		FROM tariffs WHERE is_active = true ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ts []*Tariff
	for rows.Next() {
		t := &Tariff{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Price, &t.GensCount, &t.IsActive, &t.SortOrder, &t.CreatedAt); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, nil
}

func (r *TariffRepo) List(ctx context.Context) ([]*Tariff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, COALESCE(description,''), price, gens_count, is_active, sort_order, created_at
		FROM tariffs ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ts []*Tariff
	for rows.Next() {
		t := &Tariff{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Price, &t.GensCount, &t.IsActive, &t.SortOrder, &t.CreatedAt); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, nil
}

func (r *TariffRepo) GetByID(ctx context.Context, id int) (*Tariff, error) {
	t := &Tariff{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, COALESCE(description,''), price, gens_count, is_active, sort_order, created_at
		FROM tariffs WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Description, &t.Price, &t.GensCount, &t.IsActive, &t.SortOrder, &t.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func (r *TariffRepo) Create(ctx context.Context, name, desc string, price float64, gens, sort int) (*Tariff, error) {
	t := &Tariff{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO tariffs (name, description, price, gens_count, sort_order)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, COALESCE(description,''), price, gens_count, is_active, sort_order, created_at`,
		name, desc, price, gens, sort).
		Scan(&t.ID, &t.Name, &t.Description, &t.Price, &t.GensCount, &t.IsActive, &t.SortOrder, &t.CreatedAt)
	return t, err
}

func (r *TariffRepo) Update(ctx context.Context, id int, name, desc string, price float64, gens, sort int, active bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tariffs SET name=$2, description=$3, price=$4, gens_count=$5, sort_order=$6, is_active=$7
		WHERE id=$1`, id, name, desc, price, gens, sort, active)
	return err
}

func (r *TariffRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tariffs WHERE id = $1`, id)
	return err
}

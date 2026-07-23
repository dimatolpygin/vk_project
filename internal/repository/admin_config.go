package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminConfigRepo — доступ к admin_config для компонентов, у которых нет пула напрямую.
type AdminConfigRepo struct {
	db *pgxpool.Pool
}

func NewAdminConfigRepo(db *pgxpool.Pool) *AdminConfigRepo {
	return &AdminConfigRepo{db: db}
}

func (r *AdminConfigRepo) Get(ctx context.Context, key string) (string, error) {
	return GetAdminConfig(ctx, r.db, key)
}

func (r *AdminConfigRepo) Set(ctx context.Context, key, value string) error {
	return SetAdminConfig(ctx, r.db, key, value)
}

func GetAdminConfig(ctx context.Context, db *pgxpool.Pool, key string) (string, error) {
	var value string
	err := db.QueryRow(ctx, "SELECT value FROM admin_config WHERE key = $1", key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func SetAdminConfig(ctx context.Context, db *pgxpool.Pool, key, value string) error {
	_, err := db.Exec(ctx,
		"INSERT INTO admin_config (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value",
		key, value,
	)
	return err
}

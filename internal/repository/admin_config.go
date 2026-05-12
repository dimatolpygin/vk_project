package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

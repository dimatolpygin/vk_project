package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func New(ctx context.Context, dsn string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("не удалось подключиться к PostgreSQL")
	}
	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("PostgreSQL не отвечает на ping")
	}
	log.Info().Msg("PostgreSQL подключён")
	return pool
}

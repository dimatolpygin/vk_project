package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Stats struct {
	TotalUsers      int
	TodayUsers      int
	TotalGens       int
	TodayGens       int
	TotalRevenue    float64
	TodayRevenue    float64
	TotalReferrals  int
}

type ButtonClick struct {
	Payload string
	Count   int
}

type StatsRepo struct {
	db *pgxpool.Pool
}

func NewStatsRepo(db *pgxpool.Pool) *StatsRepo {
	return &StatsRepo{db: db}
}

func (r *StatsRepo) Get(ctx context.Context) (*Stats, error) {
	s := &Stats{}
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users) AS total_users,
			(SELECT COUNT(*) FROM users WHERE created_at >= current_date) AS today_users,
			(SELECT COUNT(*) FROM generations) AS total_gens,
			(SELECT COUNT(*) FROM generations WHERE created_at >= current_date) AS today_gens,
			(SELECT COALESCE(SUM(amount),0) FROM orders WHERE status='succeeded') AS total_revenue,
			(SELECT COALESCE(SUM(amount),0) FROM orders WHERE status='succeeded' AND created_at >= current_date) AS today_revenue,
			(SELECT COUNT(*) FROM referrals) AS total_referrals`).
		Scan(&s.TotalUsers, &s.TodayUsers, &s.TotalGens, &s.TodayGens,
			&s.TotalRevenue, &s.TodayRevenue, &s.TotalReferrals)
	return s, err
}

func (r *StatsRepo) TopButtons(ctx context.Context, limit int) ([]ButtonClick, error) {
	rows, err := r.db.Query(ctx, `
		SELECT payload, COUNT(*) AS cnt FROM button_clicks
		GROUP BY payload ORDER BY cnt DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clicks []ButtonClick
	for rows.Next() {
		var bc ButtonClick
		if err := rows.Scan(&bc.Payload, &bc.Count); err != nil {
			return nil, err
		}
		clicks = append(clicks, bc)
	}
	return clicks, nil
}

func (r *StatsRepo) RecordClick(ctx context.Context, vkID int64, payload string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO button_clicks (user_vk_id, payload) VALUES ($1, $2)`, vkID, payload)
	return err
}

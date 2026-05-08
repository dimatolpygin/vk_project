package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardStats struct {
	TotalUsers      int
	PeriodUsers     int
	ActiveUsers     int
	TotalGens       int
	PeriodGens      int
	TotalRevenue    float64
	PeriodRevenue   float64
	TotalPaidUsers  int
	PeriodPayments  int
	TotalReferrals  int
	PeriodReferrals int
}

type DailyCount struct {
	Day   time.Time
	Count int
}

type DailyAmount struct {
	Day    time.Time
	Amount float64
}

type ActionCount struct {
	ActionKey string
	Count     int
}

type StatsRepo struct {
	db *pgxpool.Pool
}

func NewStatsRepo(db *pgxpool.Pool) *StatsRepo {
	return &StatsRepo{db: db}
}

func (r *StatsRepo) Dashboard(ctx context.Context, since time.Time) (*DashboardStats, error) {
	stats := &DashboardStats{}
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users) AS total_users,
			(SELECT COUNT(*) FROM users WHERE created_at >= $1) AS period_users,
			(
				SELECT COUNT(DISTINCT user_vk_id)
				FROM (
					SELECT user_vk_id FROM activity_events WHERE created_at >= $1
					UNION ALL
					SELECT user_vk_id FROM button_clicks WHERE created_at >= $1
					UNION ALL
					SELECT user_vk_id FROM generations WHERE created_at >= $1
					UNION ALL
					SELECT user_vk_id FROM orders WHERE created_at >= $1
					UNION ALL
					SELECT vk_id AS user_vk_id FROM users WHERE created_at >= $1
				) AS active_users
			) AS active_users,
			(SELECT COUNT(*) FROM generations) AS total_gens,
			(SELECT COUNT(*) FROM generations WHERE created_at >= $1) AS period_gens,
			(SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 'succeeded') AS total_revenue,
			(SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 'succeeded' AND created_at >= $1) AS period_revenue,
			(SELECT COUNT(*) FROM users WHERE status = 'paid') AS total_paid_users,
			(SELECT COUNT(*) FROM orders WHERE status = 'succeeded' AND created_at >= $1) AS period_payments,
			(SELECT COUNT(*) FROM referrals) AS total_referrals,
			(SELECT COUNT(*) FROM referrals WHERE created_at >= $1) AS period_referrals`,
		since,
	).Scan(
		&stats.TotalUsers,
		&stats.PeriodUsers,
		&stats.ActiveUsers,
		&stats.TotalGens,
		&stats.PeriodGens,
		&stats.TotalRevenue,
		&stats.PeriodRevenue,
		&stats.TotalPaidUsers,
		&stats.PeriodPayments,
		&stats.TotalReferrals,
		&stats.PeriodReferrals,
	)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *StatsRepo) RegistrationSeries(ctx context.Context, since time.Time) ([]DailyCount, error) {
	return r.dailyCounts(ctx, `
		SELECT created_at::date AS day, COUNT(*) AS count
		FROM users
		WHERE created_at >= $1
		GROUP BY day
		ORDER BY day`, since)
}

func (r *StatsRepo) GenerationSeries(ctx context.Context, since time.Time) ([]DailyCount, error) {
	return r.dailyCounts(ctx, `
		SELECT created_at::date AS day, COUNT(*) AS count
		FROM generations
		WHERE created_at >= $1
		GROUP BY day
		ORDER BY day`, since)
}

func (r *StatsRepo) RevenueSeries(ctx context.Context, since time.Time) ([]DailyAmount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT created_at::date AS day, COALESCE(SUM(amount), 0) AS amount
		FROM orders
		WHERE status = 'succeeded' AND created_at >= $1
		GROUP BY day
		ORDER BY day`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []DailyAmount
	for rows.Next() {
		var point DailyAmount
		if err := rows.Scan(&point.Day, &point.Amount); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (r *StatsRepo) ActiveUsersSeries(ctx context.Context, since time.Time) ([]DailyCount, error) {
	return r.dailyCounts(ctx, `
		SELECT day, COUNT(DISTINCT user_vk_id) AS count
		FROM (
			SELECT created_at::date AS day, user_vk_id FROM activity_events WHERE created_at >= $1
			UNION ALL
			SELECT created_at::date AS day, user_vk_id FROM button_clicks WHERE created_at >= $1
			UNION ALL
			SELECT created_at::date AS day, user_vk_id FROM generations WHERE created_at >= $1
			UNION ALL
			SELECT created_at::date AS day, user_vk_id FROM orders WHERE created_at >= $1
			UNION ALL
			SELECT created_at::date AS day, vk_id AS user_vk_id FROM users WHERE created_at >= $1
		) AS events
		GROUP BY day
		ORDER BY day`, since)
}

func (r *StatsRepo) ActionCounts(ctx context.Context, since time.Time) ([]ActionCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT payload, COUNT(*) AS count
		FROM button_clicks
		WHERE created_at >= $1
		GROUP BY payload
		ORDER BY count DESC, payload ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []ActionCount
	for rows.Next() {
		var item ActionCount
		if err := rows.Scan(&item.ActionKey, &item.Count); err != nil {
			return nil, err
		}
		counts = append(counts, item)
	}
	return counts, rows.Err()
}

func (r *StatsRepo) RecordClick(ctx context.Context, vkID int64, payload string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO button_clicks (user_vk_id, payload) VALUES ($1, $2)`, vkID, payload)
	return err
}

func (r *StatsRepo) dailyCounts(ctx context.Context, query string, since time.Time) ([]DailyCount, error) {
	rows, err := r.db.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []DailyCount
	for rows.Next() {
		var point DailyCount
		if err := rows.Scan(&point.Day, &point.Count); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

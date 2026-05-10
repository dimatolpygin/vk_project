package repository

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID              int64
	VKID            int64
	Username        string
	FirstName       string
	Gender          string
	ReferralCode    string
	ReferredBy      *int64
	FreeGens        int
	PaidGens        int
	Subscribed      bool
	SavedPhotoURL   *string
	UseSavedPhoto   bool
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	PrefModel       string
	PrefResolution  string
	PrefAspectRatio string
}

type AdminUserListParams struct {
	Query  string
	Status string
	Limit  int
	Offset int
}

type AdminUserSummary struct {
	TotalUsers    int
	PaidUsers     int
	NewUsersToday int
	ActiveUsers7d int
}

type AdminUserListItem struct {
	User
	LastActivity time.Time
}

type AdminUserDetail struct {
	User
	LastActivity       time.Time
	GeneratedCount     int
	CompletedGenCount  int
	ReferredCount      int
	ReferralBonusCount int
	OrdersCount        int
	SuccessOrdersCount int
	TotalSpent         float64
}

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetByVKID(ctx context.Context, vkID int64) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, vk_id, COALESCE(username,''), COALESCE(first_name,''), gender, referral_code,
		       referred_by, free_gens, paid_gens, subscribed, saved_photo_url, use_saved_photo, status,
		       created_at, updated_at, pref_model, pref_resolution, pref_aspect_ratio
		FROM users WHERE vk_id = $1`, vkID).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.UseSavedPhoto,
			&u.Status, &u.CreatedAt, &u.UpdatedAt,
			&u.PrefModel, &u.PrefResolution, &u.PrefAspectRatio)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) Create(ctx context.Context, vkID int64, username, firstName string, referredBy *int64) (*User, error) {
	code := genReferralCode()
	u := &User{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (vk_id, username, first_name, referral_code, referred_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, vk_id, COALESCE(username,''), COALESCE(first_name,''), gender, referral_code,
		          referred_by, free_gens, paid_gens, subscribed, saved_photo_url, use_saved_photo, status,
		          created_at, updated_at, pref_model, pref_resolution, pref_aspect_ratio`,
		vkID, username, firstName, code, referredBy).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.UseSavedPhoto,
			&u.Status, &u.CreatedAt, &u.UpdatedAt,
			&u.PrefModel, &u.PrefResolution, &u.PrefAspectRatio)
	return u, err
}

func (r *UserRepo) CreateWithReferralCode(ctx context.Context, vkID int64, username, firstName, referralCode string) (*User, error) {
	referralCode = strings.TrimSpace(referralCode)
	if referralCode == "" {
		return r.Create(ctx, vkID, username, firstName, nil)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		referredBy   *int64
		referrerVKID int64
	)
	err = tx.QueryRow(ctx, `SELECT vk_id FROM users WHERE referral_code = $1`, referralCode).Scan(&referrerVKID)
	switch {
	case err == nil && referrerVKID != vkID:
		referredBy = &referrerVKID
	case err == pgx.ErrNoRows:
		referredBy = nil
	case err != nil:
		return nil, err
	}

	code := genReferralCode()
	u := &User{}
	err = tx.QueryRow(ctx, `
		INSERT INTO users (vk_id, username, first_name, referral_code, referred_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, vk_id, COALESCE(username,''), COALESCE(first_name,''), gender, referral_code,
		          referred_by, free_gens, paid_gens, subscribed, saved_photo_url, use_saved_photo, status,
		          created_at, updated_at, pref_model, pref_resolution, pref_aspect_ratio`,
		vkID, username, firstName, code, referredBy).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.UseSavedPhoto,
			&u.Status, &u.CreatedAt, &u.UpdatedAt,
			&u.PrefModel, &u.PrefResolution, &u.PrefAspectRatio)
	if err != nil {
		return nil, err
	}

	if referredBy != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO referrals (referrer_vk_id, referred_vk_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			referrerVKID, vkID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) GetByReferralCode(ctx context.Context, code string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, vk_id, COALESCE(username,''), COALESCE(first_name,''), gender, referral_code,
		       referred_by, free_gens, paid_gens, subscribed, saved_photo_url, use_saved_photo, status,
		       created_at, updated_at, pref_model, pref_resolution, pref_aspect_ratio
		FROM users WHERE referral_code = $1`, code).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.UseSavedPhoto,
			&u.Status, &u.CreatedAt, &u.UpdatedAt,
			&u.PrefModel, &u.PrefResolution, &u.PrefAspectRatio)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) AddPaidGens(ctx context.Context, vkID int64, count int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET paid_gens = paid_gens + $2, status = 'paid', updated_at = now()
		WHERE vk_id = $1`, vkID, count)
	return err
}

func (r *UserRepo) AddFreeGens(ctx context.Context, vkID int64, count int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET free_gens = free_gens + $2, updated_at = now()
		WHERE vk_id = $1`, vkID, count)
	return err
}

func (r *UserRepo) SetGender(ctx context.Context, vkID int64, gender string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET gender = $2, updated_at = now() WHERE vk_id = $1`, vkID, gender)
	return err
}

func (r *UserRepo) SetSubscribed(ctx context.Context, vkID int64, subscribed bool) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET subscribed = $2, updated_at = now() WHERE vk_id = $1`, vkID, subscribed)
	return err
}

func (r *UserRepo) SetSavedPhoto(ctx context.Context, vkID int64, url string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET saved_photo_url = $2, updated_at = now() WHERE vk_id = $1`, vkID, url)
	return err
}

func (r *UserRepo) SetUseSavedPhoto(ctx context.Context, vkID int64, enabled bool) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET use_saved_photo = $2, updated_at = now() WHERE vk_id = $1`, vkID, enabled)
	return err
}

func (r *UserRepo) HasGens(ctx context.Context, vkID int64) (bool, error) {
	var freeGens, paidGens int
	err := r.db.QueryRow(ctx, `SELECT free_gens, paid_gens FROM users WHERE vk_id = $1`, vkID).
		Scan(&freeGens, &paidGens)
	if err != nil {
		return false, err
	}
	return freeGens > 0 || paidGens > 0, nil
}

func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]*User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, vk_id, COALESCE(username,''), COALESCE(first_name,''), gender, referral_code,
		       referred_by, free_gens, paid_gens, subscribed, saved_photo_url, use_saved_photo, status,
		       created_at, updated_at, pref_model, pref_resolution, pref_aspect_ratio
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.UseSavedPhoto,
			&u.Status, &u.CreatedAt, &u.UpdatedAt,
			&u.PrefModel, &u.PrefResolution, &u.PrefAspectRatio); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepo) SaveSettings(ctx context.Context, vkID int64, model, resolution, aspectRatio string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET pref_model=$2, pref_resolution=$3, pref_aspect_ratio=$4, updated_at=now()
		WHERE vk_id=$1`, vkID, model, resolution, aspectRatio)
	return err
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *UserRepo) AdminSummary(ctx context.Context, now time.Time) (*AdminUserSummary, error) {
	summary := &AdminUserSummary{}
	since := now.Add(-7 * 24 * time.Hour)

	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users) AS total_users,
			(SELECT COUNT(*) FROM users WHERE status = 'paid') AS paid_users,
			(SELECT COUNT(*) FROM users WHERE created_at >= current_date) AS new_users_today,
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
			) AS active_users_7d`,
		since,
	).Scan(&summary.TotalUsers, &summary.PaidUsers, &summary.NewUsersToday, &summary.ActiveUsers7d)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (r *UserRepo) AdminCount(ctx context.Context, params AdminUserListParams) (int, error) {
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE ($1 = '' OR vk_id::text ILIKE '%' || $1 || '%' OR COALESCE(username, '') ILIKE '%' || $1 || '%' OR COALESCE(first_name, '') ILIKE '%' || $1 || '%' OR COALESCE(referral_code, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR status = $2)`,
		params.Query, params.Status,
	).Scan(&total)
	return total, err
}

func (r *UserRepo) AdminList(ctx context.Context, params AdminUserListParams) ([]*AdminUserListItem, error) {
	rows, err := r.db.Query(ctx, `
		WITH activity AS (
			SELECT user_vk_id, MAX(event_at) AS last_activity
			FROM (
				SELECT user_vk_id, created_at AS event_at FROM activity_events
				UNION ALL
				SELECT user_vk_id, created_at AS event_at FROM button_clicks
				UNION ALL
				SELECT user_vk_id, created_at AS event_at FROM generations
				UNION ALL
				SELECT user_vk_id, created_at AS event_at FROM orders
				UNION ALL
				SELECT vk_id AS user_vk_id, updated_at AS event_at FROM users
			) AS events
			GROUP BY user_vk_id
		)
		SELECT
			u.id, u.vk_id, COALESCE(u.username, ''), COALESCE(u.first_name, ''), u.gender, u.referral_code,
			u.referred_by, u.free_gens, u.paid_gens, u.subscribed, u.saved_photo_url, u.use_saved_photo, u.status,
			u.created_at, u.updated_at, u.pref_model, u.pref_resolution, u.pref_aspect_ratio,
			COALESCE(activity.last_activity, u.updated_at) AS last_activity
		FROM users u
		LEFT JOIN activity ON activity.user_vk_id = u.vk_id
		WHERE ($1 = '' OR u.vk_id::text ILIKE '%' || $1 || '%' OR COALESCE(u.username, '') ILIKE '%' || $1 || '%' OR COALESCE(u.first_name, '') ILIKE '%' || $1 || '%' OR COALESCE(u.referral_code, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR u.status = $2)
		ORDER BY COALESCE(activity.last_activity, u.updated_at) DESC, u.created_at DESC
		LIMIT $3 OFFSET $4`,
		params.Query, params.Status, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*AdminUserListItem
	for rows.Next() {
		item := &AdminUserListItem{}
		if err := rows.Scan(
			&item.ID, &item.VKID, &item.Username, &item.FirstName, &item.Gender, &item.ReferralCode,
			&item.ReferredBy, &item.FreeGens, &item.PaidGens, &item.Subscribed, &item.SavedPhotoURL, &item.UseSavedPhoto,
			&item.Status, &item.CreatedAt, &item.UpdatedAt, &item.PrefModel, &item.PrefResolution, &item.PrefAspectRatio,
			&item.LastActivity,
		); err != nil {
			return nil, err
		}
		users = append(users, item)
	}

	return users, rows.Err()
}

func (r *UserRepo) AdminDetail(ctx context.Context, vkID int64) (*AdminUserDetail, error) {
	detail := &AdminUserDetail{}
	err := r.db.QueryRow(ctx, `
		SELECT
			u.id, u.vk_id, COALESCE(u.username, ''), COALESCE(u.first_name, ''), u.gender, u.referral_code,
			u.referred_by, u.free_gens, u.paid_gens, u.subscribed, u.saved_photo_url, u.use_saved_photo, u.status,
			u.created_at, u.updated_at, u.pref_model, u.pref_resolution, u.pref_aspect_ratio,
			COALESCE(activity.last_activity, u.updated_at) AS last_activity,
			COALESCE(gen_summary.generated_count, 0) AS generated_count,
			COALESCE(gen_summary.completed_count, 0) AS completed_count,
			COALESCE(ref_summary.referred_count, 0) AS referred_count,
			COALESCE(ref_summary.bonus_count, 0) AS bonus_count,
			COALESCE(order_summary.orders_count, 0) AS orders_count,
			COALESCE(order_summary.success_orders_count, 0) AS success_orders_count,
			COALESCE(order_summary.total_spent, 0) AS total_spent
		FROM users u
		LEFT JOIN LATERAL (
			SELECT MAX(event_at) AS last_activity
			FROM (
				SELECT created_at AS event_at FROM activity_events WHERE user_vk_id = u.vk_id
				UNION ALL
				SELECT created_at AS event_at FROM button_clicks WHERE user_vk_id = u.vk_id
				UNION ALL
				SELECT created_at AS event_at FROM generations WHERE user_vk_id = u.vk_id
				UNION ALL
				SELECT created_at AS event_at FROM orders WHERE user_vk_id = u.vk_id
				UNION ALL
				SELECT updated_at AS event_at FROM users WHERE vk_id = u.vk_id
			) AS events
		) AS activity ON true
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS generated_count,
				COUNT(*) FILTER (WHERE status = 'completed') AS completed_count
			FROM generations
			WHERE user_vk_id = u.vk_id
		) AS gen_summary ON true
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS referred_count,
				COUNT(*) FILTER (WHERE bonus_given) AS bonus_count
			FROM referrals
			WHERE referrer_vk_id = u.vk_id
		) AS ref_summary ON true
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS orders_count,
				COUNT(*) FILTER (WHERE status = 'succeeded') AS success_orders_count,
				COALESCE(SUM(amount) FILTER (WHERE status = 'succeeded'), 0) AS total_spent
			FROM orders
			WHERE user_vk_id = u.vk_id
		) AS order_summary ON true
		WHERE u.vk_id = $1`,
		vkID,
	).Scan(
		&detail.ID, &detail.VKID, &detail.Username, &detail.FirstName, &detail.Gender, &detail.ReferralCode,
		&detail.ReferredBy, &detail.FreeGens, &detail.PaidGens, &detail.Subscribed, &detail.SavedPhotoURL, &detail.UseSavedPhoto,
		&detail.Status, &detail.CreatedAt, &detail.UpdatedAt, &detail.PrefModel, &detail.PrefResolution, &detail.PrefAspectRatio,
		&detail.LastActivity, &detail.GeneratedCount, &detail.CompletedGenCount, &detail.ReferredCount, &detail.ReferralBonusCount,
		&detail.OrdersCount, &detail.SuccessOrdersCount, &detail.TotalSpent,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return detail, err
}

func (r *UserRepo) Delete(ctx context.Context, vkID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, _ = tx.Exec(ctx, `UPDATE users SET referred_by = NULL, updated_at = now() WHERE referred_by=$1`, vkID)
	_, _ = tx.Exec(ctx, `DELETE FROM referrals  WHERE referrer_vk_id=$1 OR referred_vk_id=$1`, vkID)
	_, _ = tx.Exec(ctx, `DELETE FROM generations WHERE user_vk_id=$1`, vkID)
	_, _ = tx.Exec(ctx, `DELETE FROM orders      WHERE user_vk_id=$1`, vkID)
	_, err = tx.Exec(ctx, `DELETE FROM users WHERE vk_id=$1`, vkID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func genReferralCode() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("ref_%s", string(b))
}

package repository

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID            int64
	VKID          int64
	Username      string
	FirstName     string
	Gender        string
	ReferralCode  string
	ReferredBy    *int64
	FreeGens      int
	PaidGens      int
	Subscribed    bool
	SavedPhotoURL *string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
		       referred_by, free_gens, paid_gens, subscribed, saved_photo_url, status, created_at, updated_at
		FROM users WHERE vk_id = $1`, vkID).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.Status,
			&u.CreatedAt, &u.UpdatedAt)
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
		          referred_by, free_gens, paid_gens, subscribed, saved_photo_url, status, created_at, updated_at`,
		vkID, username, firstName, code, referredBy).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.Status,
			&u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *UserRepo) GetByReferralCode(ctx context.Context, code string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, vk_id, COALESCE(username,''), COALESCE(first_name,''), gender, referral_code,
		       referred_by, free_gens, paid_gens, subscribed, saved_photo_url, status, created_at, updated_at
		FROM users WHERE referral_code = $1`, code).
		Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.Status,
			&u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) DecrementGens(ctx context.Context, vkID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
			free_gens  = CASE WHEN paid_gens = 0 AND free_gens > 0 THEN free_gens - 1 ELSE free_gens END,
			paid_gens  = CASE WHEN paid_gens > 0 THEN paid_gens - 1 ELSE paid_gens END,
			updated_at = now()
		WHERE vk_id = $1`, vkID)
	return err
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
		       referred_by, free_gens, paid_gens, subscribed, saved_photo_url, status, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.VKID, &u.Username, &u.FirstName, &u.Gender, &u.ReferralCode,
			&u.ReferredBy, &u.FreeGens, &u.PaidGens, &u.Subscribed, &u.SavedPhotoURL, &u.Status,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *UserRepo) Delete(ctx context.Context, vkID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

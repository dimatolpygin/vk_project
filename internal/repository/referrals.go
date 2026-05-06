package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Referral struct {
	ID            int64
	ReferrerVKID  int64
	ReferredVKID  int64
	BonusGiven    bool
	CreatedAt     time.Time
}

type ReferralRepo struct {
	db *pgxpool.Pool
}

func NewReferralRepo(db *pgxpool.Pool) *ReferralRepo {
	return &ReferralRepo{db: db}
}

func (r *ReferralRepo) Create(ctx context.Context, referrerVKID, referredVKID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO referrals (referrer_vk_id, referred_vk_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		referrerVKID, referredVKID)
	return err
}

func (r *ReferralRepo) GiveBonus(ctx context.Context, referredVKID int64) (*Referral, error) {
	ref := &Referral{}
	err := r.db.QueryRow(ctx, `
		UPDATE referrals SET bonus_given = true
		WHERE referred_vk_id = $1 AND bonus_given = false
		RETURNING id, referrer_vk_id, referred_vk_id, bonus_given, created_at`,
		referredVKID).
		Scan(&ref.ID, &ref.ReferrerVKID, &ref.ReferredVKID, &ref.BonusGiven, &ref.CreatedAt)
	return ref, err
}

func (r *ReferralRepo) CountByReferrer(ctx context.Context, referrerVKID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM referrals WHERE referrer_vk_id = $1`, referrerVKID).Scan(&n)
	return n, err
}

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Order struct {
	ID               int64
	UserVKID         int64
	TariffID         int
	YukassaPaymentID *string
	Amount           float64
	Status           string
	CreatedAt        time.Time
}

type PaymentSettlementResult struct {
	PaymentID        string
	UserVKID         int64
	TariffID         int
	PaidGensAdded    int
	BonusGranted     bool
	ReferrerVKID     int64
	AlreadyProcessed bool
}

type OrderRepo struct {
	db *pgxpool.Pool
}

func NewOrderRepo(db *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Create(ctx context.Context, userVKID int64, tariffID int, amount float64) (*Order, error) {
	o := &Order{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO orders (user_vk_id, tariff_id, amount)
		VALUES ($1, $2, $3)
		RETURNING id, user_vk_id, tariff_id, yukassa_payment_id, amount, status, created_at`,
		userVKID, tariffID, amount).
		Scan(&o.ID, &o.UserVKID, &o.TariffID, &o.YukassaPaymentID, &o.Amount, &o.Status, &o.CreatedAt)
	return o, err
}

func (r *OrderRepo) SetPaymentID(ctx context.Context, orderID int64, paymentID string) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET yukassa_payment_id = $2 WHERE id = $1`, orderID, paymentID)
	return err
}

func (r *OrderRepo) SetStatus(ctx context.Context, paymentID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET status = $2 WHERE yukassa_payment_id = $1`, paymentID, status)
	return err
}

func (r *OrderRepo) SettleSuccessfulPayment(ctx context.Context, paymentID string, userVKID int64, tariffID int) (*PaymentSettlementResult, error) {
	const referralBonusGens = 2

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var order Order
	if err := tx.QueryRow(ctx, `
		SELECT id, user_vk_id, tariff_id, yukassa_payment_id, amount, status, created_at
		FROM orders
		WHERE yukassa_payment_id = $1
		FOR UPDATE`, paymentID).
		Scan(&order.ID, &order.UserVKID, &order.TariffID, &order.YukassaPaymentID, &order.Amount, &order.Status, &order.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order not found for payment %s", paymentID)
		}
		return nil, err
	}

	if userVKID > 0 && order.UserVKID != userVKID {
		return nil, fmt.Errorf("payment %s belongs to user %d, webhook user %d", paymentID, order.UserVKID, userVKID)
	}
	if tariffID > 0 && order.TariffID != tariffID {
		return nil, fmt.Errorf("payment %s belongs to tariff %d, webhook tariff %d", paymentID, order.TariffID, tariffID)
	}

	result := &PaymentSettlementResult{
		PaymentID: paymentID,
		UserVKID:  order.UserVKID,
		TariffID:  order.TariffID,
	}
	if order.Status == "succeeded" {
		result.AlreadyProcessed = true
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}

	if err := tx.QueryRow(ctx, `SELECT gens_count FROM tariffs WHERE id = $1`, order.TariffID).Scan(&result.PaidGensAdded); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tariff %d not found for payment %s", order.TariffID, paymentID)
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'succeeded' WHERE yukassa_payment_id = $1`, paymentID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET paid_gens = paid_gens + $2, status = 'paid', updated_at = now()
		WHERE vk_id = $1`, order.UserVKID, result.PaidGensAdded); err != nil {
		return nil, err
	}

	err = tx.QueryRow(ctx, `
		UPDATE referrals
		SET bonus_given = true
		WHERE referred_vk_id = $1 AND bonus_given = false
		RETURNING referrer_vk_id`, order.UserVKID).
		Scan(&result.ReferrerVKID)
	switch {
	case err == nil:
		result.BonusGranted = true
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET free_gens = free_gens + $2, updated_at = now()
			WHERE vk_id = $1`, result.ReferrerVKID, referralBonusGens); err != nil {
			return nil, err
		}
	case err == pgx.ErrNoRows:
		// no referral bonus for this payment
	default:
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *OrderRepo) GetByPaymentID(ctx context.Context, paymentID string) (*Order, error) {
	o := &Order{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_vk_id, tariff_id, yukassa_payment_id, amount, status, created_at
		FROM orders WHERE yukassa_payment_id = $1`, paymentID).
		Scan(&o.ID, &o.UserVKID, &o.TariffID, &o.YukassaPaymentID, &o.Amount, &o.Status, &o.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return o, err
}

func (r *OrderRepo) ListByUser(ctx context.Context, vkID int64) ([]*Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_vk_id, tariff_id, yukassa_payment_id, amount, status, created_at
		FROM orders WHERE user_vk_id = $1 ORDER BY created_at DESC`, vkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []*Order
	for rows.Next() {
		o := &Order{}
		if err := rows.Scan(&o.ID, &o.UserVKID, &o.TariffID, &o.YukassaPaymentID, &o.Amount, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (r *OrderRepo) TotalRevenue(ctx context.Context) (float64, error) {
	var total float64
	err := r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 'succeeded'`).Scan(&total)
	return total, err
}

func (r *OrderRepo) TodayRevenue(ctx context.Context) (float64, error) {
	var total float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM orders
		WHERE status = 'succeeded' AND created_at >= current_date`).Scan(&total)
	return total, err
}

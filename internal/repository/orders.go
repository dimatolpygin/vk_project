package repository

import (
	"context"
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

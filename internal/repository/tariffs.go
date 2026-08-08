package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultVideoCostGens — запасная цена видео, если пакет в тарифах не помечен.
// Считалась из себестоимости на этапе 10: $2.46 за ролик при 17.25 ₽ за
// генерацию. Используется только чтобы видео не стало бесплатным, если галочку
// сняли или пакет удалили.
const DefaultVideoCostGens = 40

type Tariff struct {
	ID          int
	Name        string
	Description string
	Price       float64
	GensCount   int
	IsActive    bool
	SortOrder   int
	// IsVideoPack — пакет ровно на одно видео. Его GensCount и есть цена
	// видео-генерации: единственный источник истины для списания.
	IsVideoPack bool
	CreatedAt   time.Time
}

// TariffInput — поля тарифа, которые правит админка.
type TariffInput struct {
	Name        string
	Description string
	Price       float64
	GensCount   int
	SortOrder   int
	IsActive    bool
	IsVideoPack bool
}

type TariffRepo struct {
	db *pgxpool.Pool
}

func NewTariffRepo(db *pgxpool.Pool) *TariffRepo {
	return &TariffRepo{db: db}
}

const tariffColumns = `id, name, COALESCE(description,''), price, gens_count, is_active, sort_order,
	       COALESCE(is_video_pack, false), created_at`

func scanTariff(row rowScanner, t *Tariff) error {
	return row.Scan(&t.ID, &t.Name, &t.Description, &t.Price, &t.GensCount, &t.IsActive,
		&t.SortOrder, &t.IsVideoPack, &t.CreatedAt)
}

func scanTariffs(rows pgx.Rows) ([]*Tariff, error) {
	var ts []*Tariff
	for rows.Next() {
		t := &Tariff{}
		if err := scanTariff(rows, t); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, nil
}

func (r *TariffRepo) ListActive(ctx context.Context) ([]*Tariff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+tariffColumns+`
		FROM tariffs WHERE is_active = true ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTariffs(rows)
}

func (r *TariffRepo) List(ctx context.Context) ([]*Tariff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+tariffColumns+`
		FROM tariffs ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTariffs(rows)
}

func (r *TariffRepo) GetByID(ctx context.Context, id int) (*Tariff, error) {
	t := &Tariff{}
	err := scanTariff(r.db.QueryRow(ctx, `
		SELECT `+tariffColumns+`
		FROM tariffs WHERE id = $1`, id), t)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// VideoCostGens — сколько генераций стоит одно видео. Берётся из тарифа,
// помеченного как видеопакет: пакет равен одному видео, значит его объём и
// есть цена. Пакета нет — возвращаем запасное значение, чтобы видео не
// раздавалось бесплатно.
func (r *TariffRepo) VideoCostGens(ctx context.Context) (int, error) {
	var gens int
	err := r.db.QueryRow(ctx,
		`SELECT gens_count FROM tariffs WHERE is_video_pack LIMIT 1`).Scan(&gens)
	if err == pgx.ErrNoRows {
		return DefaultVideoCostGens, nil
	}
	if err != nil {
		return DefaultVideoCostGens, err
	}
	if gens < 1 {
		return DefaultVideoCostGens, nil
	}
	return gens, nil
}

func (r *TariffRepo) Create(ctx context.Context, in TariffInput) (*Tariff, error) {
	t := &Tariff{}
	err := r.inTx(ctx, in.IsVideoPack, 0, func(tx pgx.Tx) error {
		return scanTariff(tx.QueryRow(ctx, `
			INSERT INTO tariffs (name, description, price, gens_count, sort_order, is_video_pack)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+tariffColumns,
			in.Name, in.Description, in.Price, in.GensCount, in.SortOrder, in.IsVideoPack), t)
	})
	return t, err
}

func (r *TariffRepo) Update(ctx context.Context, id int, in TariffInput) error {
	return r.inTx(ctx, in.IsVideoPack, id, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE tariffs SET name=$2, description=$3, price=$4, gens_count=$5, sort_order=$6,
			    is_active=$7, is_video_pack=$8
			WHERE id=$1`,
			id, in.Name, in.Description, in.Price, in.GensCount, in.SortOrder, in.IsActive, in.IsVideoPack)
		return err
	})
}

// inTx снимает галочку видеопакета с прочих тарифов перед записью: пакет
// один, и админ, поставивший галочку новому тарифу, ожидает переноса, а не
// ошибки уникального индекса.
func (r *TariffRepo) inTx(ctx context.Context, clearOthers bool, keepID int, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if clearOthers {
		if _, err := tx.Exec(ctx,
			`UPDATE tariffs SET is_video_pack = false WHERE is_video_pack AND id <> $1`, keepID); err != nil {
			return err
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *TariffRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tariffs WHERE id = $1`, id)
	return err
}

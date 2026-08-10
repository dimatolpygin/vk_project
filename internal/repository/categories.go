package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Category struct {
	ID         int
	Name       string
	GroupID    *int
	PreviewURL *string
	Gender     string
	SortOrder  int
	IsActive   bool
	// Дерево разделов (миграция 00029).
	ParentID *int
	Section  string
	// ScreenKey — экран (текст + картинка), который показывается при входе в узел,
	// когда у узла есть подменю. Пустой означает «взять экран раздела».
	ScreenKey *string
	// PromptsScreenKey — экран над списком промтов узла, PhotoScreenKey — экран
	// запроса фото после выбора промта (миграция 00039). Оба наследуются вниз по
	// дереву, как PromptGender: текст задают на «Мальчике», а промты лежат
	// в его подкатегориях. NULL — экран раздела по умолчанию.
	PromptsScreenKey *string
	PhotoScreenKey   *string
	MediaKind        string
	// PromptGender — пол, которым фильтруются промты узла, вместо пола пользователя.
	// NULL наследуется от ближайшего предка, у которого он задан.
	PromptGender *string
}

// CategoryFilter описывает, какие узлы дерева показывать пользователю.
type CategoryFilter struct {
	// Gender — пол, по которому отбираются промты при проверке наполнения.
	Gender string
	// RequireContent прячет узлы, в которых нет ни подходящих промтов, ни активных
	// детей. Так ведёт себя раздел готовых стилей: пустая категория в нём никогда
	// не показывалась. Разделы с экраном-заглушкой «Скоро» ставят false.
	RequireContent bool
}

const categoryColumns = `id, name, group_id, preview_url, gender, sort_order, is_active,
	parent_id, section, screen_key, prompts_screen_key, photo_screen_key, media_kind, prompt_gender`

// contentPredicate — «в узле есть что показать»: подходящие промты или активные дети.
// Плейсхолдеры фиксированы: $2 — флаг RequireContent, $3 — пол. Обе выборки уровня
// (ListRoots и ListChildren) держат этот порядок аргументов.
const contentPredicate = `(
	NOT $2
	OR EXISTS (
		SELECT 1 FROM prompts AS p
		WHERE p.category_id = c.id AND p.is_active = true
		  AND (p.gender = 'any' OR p.gender = $3)
	)
	OR EXISTS (
		SELECT 1 FROM categories AS ch
		WHERE ch.parent_id = c.id AND ch.is_active = true
	)
)`

// Разделы дерева и виды медиа — держим строками в одном месте, чтобы не разъезжались
// между миграциями, репозиторием и навигатором.
const (
	SectionSelf      = "self"
	SectionCouple    = "couple"
	SectionKids      = "kids"
	SectionTrends    = "trends"
	SectionGreetings = "greetings"

	MediaKindPhoto = "photo"
	MediaKindVideo = "video"

	GenderAny    = "any"
	GenderCouple = "couple"
)

// ErrCategoryCycle возвращается, когда узел пытаются увести под собственного потомка.
var ErrCategoryCycle = errors.New("категория не может быть вложена сама в себя")

type CategoryRepo struct {
	db *pgxpool.Pool
}

func NewCategoryRepo(db *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{db: db}
}

// ListRoots возвращает корневые узлы раздела — верхний уровень меню.
func (r *CategoryRepo) ListRoots(ctx context.Context, section string, filter CategoryFilter) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+categoryColumns+`
		FROM categories AS c
		WHERE c.is_active = true
		  AND c.parent_id IS NULL
		  AND c.section = $1
		  AND `+contentPredicate+`
		ORDER BY c.sort_order, c.id`, section, filter.RequireContent, filter.Gender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

// ListChildren возвращает активных детей узла — следующий уровень меню.
func (r *CategoryRepo) ListChildren(ctx context.Context, parentID int, filter CategoryFilter) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+categoryColumns+`
		FROM categories AS c
		WHERE c.is_active = true
		  AND c.parent_id = $1
		  AND `+contentPredicate+`
		ORDER BY c.sort_order, c.id`, parentID, filter.RequireContent, filter.Gender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

// HasChildren отвечает, показывать ли в узле подменю или сразу промты.
func (r *CategoryRepo) HasChildren(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM categories WHERE parent_id = $1 AND is_active = true
		)`, id).Scan(&exists)
	return exists, err
}

// Path возвращает путь от корня до узла включительно: нужен для кнопки «Назад»
// и для наследования prompt_gender.
func (r *CategoryRepo) Path(ctx context.Context, id int) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT c.*, 0 AS depth FROM categories AS c WHERE c.id = $1
			UNION ALL
			SELECT p.*, a.depth + 1
			FROM categories AS p
			JOIN ancestors AS a ON a.parent_id = p.id
		)
		SELECT `+categoryColumns+`
		FROM ancestors AS c
		ORDER BY depth DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (r *CategoryRepo) ListReadyPromptCategories(ctx context.Context, gender string) ([]*Category, error) {
	return r.ListRoots(ctx, SectionSelf, CategoryFilter{Gender: gender, RequireContent: true})
}

func (r *CategoryRepo) ListActive(ctx context.Context, gender string) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+categoryColumns+`
		FROM categories AS c
		WHERE is_active = true AND (gender = 'any' OR gender = $1)
		ORDER BY sort_order`, gender)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (r *CategoryRepo) ListActiveCouple(ctx context.Context) ([]*Category, error) {
	return r.ListRoots(ctx, SectionCouple, CategoryFilter{Gender: GenderCouple})
}

// List отдаёт всё дерево целиком в порядке обхода сверху вниз — для админки.
func (r *CategoryRepo) List(ctx context.Context) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE tree AS (
			SELECT c.*, ARRAY[c.sort_order, c.id] AS path
			FROM categories AS c WHERE c.parent_id IS NULL
			UNION ALL
			SELECT ch.*, t.path || ch.sort_order || ch.id
			FROM categories AS ch
			JOIN tree AS t ON ch.parent_id = t.id
		)
		SELECT `+categoryColumns+`
		FROM tree AS c
		ORDER BY c.section, c.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (r *CategoryRepo) GetByID(ctx context.Context, id int) (*Category, error) {
	c := &Category{}
	err := r.db.QueryRow(ctx, `
		SELECT `+categoryColumns+`
		FROM categories AS c WHERE id = $1`, id).Scan(categoryScanTargets(c)...)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// CategoryInput — поля, которыми админка создаёт и правит узел дерева.
type CategoryInput struct {
	Name             string
	Gender           string
	SortOrder        int
	IsActive         bool
	PreviewURL       *string
	ParentID         *int
	Section          string
	ScreenKey        *string
	PromptsScreenKey *string
	PhotoScreenKey   *string
	MediaKind        string
	PromptGender     *string
}

func (in *CategoryInput) normalize() {
	if in.Gender == "" {
		in.Gender = GenderAny
	}
	if in.Section == "" {
		in.Section = SectionSelf
	}
	if in.MediaKind == "" {
		in.MediaKind = MediaKindPhoto
	}
	for _, key := range []**string{&in.ScreenKey, &in.PromptsScreenKey, &in.PhotoScreenKey} {
		if *key != nil && **key == "" {
			*key = nil
		}
	}
	if in.PromptGender != nil && *in.PromptGender == "" {
		in.PromptGender = nil
	}
}

func (r *CategoryRepo) Create(ctx context.Context, in CategoryInput) (*Category, error) {
	in.normalize()

	// Раздел ребёнка всегда совпадает с разделом родителя: узел, оторванный от
	// своего раздела, не найдётся ни одной выборкой и станет невидимым.
	if in.ParentID != nil {
		parent, err := r.GetByID(ctx, *in.ParentID)
		if err != nil {
			return nil, err
		}
		if parent != nil {
			in.Section = parent.Section
		}
	}

	c := &Category{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO categories (name, gender, sort_order, parent_id, section, screen_key,
		    prompts_screen_key, photo_screen_key, media_kind, prompt_gender)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+categoryColumns,
		in.Name, in.Gender, in.SortOrder, in.ParentID, in.Section, in.ScreenKey,
		in.PromptsScreenKey, in.PhotoScreenKey, in.MediaKind, in.PromptGender).
		Scan(categoryScanTargets(c)...)
	return c, err
}

func (r *CategoryRepo) Update(ctx context.Context, id int, in CategoryInput) error {
	in.normalize()

	if in.ParentID != nil {
		if *in.ParentID == id {
			return ErrCategoryCycle
		}
		ancestors, err := r.Path(ctx, *in.ParentID)
		if err != nil {
			return err
		}
		for _, ancestor := range ancestors {
			// Узел нельзя увести под собственного потомка — дерево свернётся в цикл,
			// и рекурсивные выборки уйдут в бесконечность.
			if ancestor.ID == id {
				return ErrCategoryCycle
			}
		}
		parent, err := r.GetByID(ctx, *in.ParentID)
		if err != nil {
			return err
		}
		if parent != nil {
			in.Section = parent.Section
		}
	}

	_, err := r.db.Exec(ctx, `
		UPDATE categories
		SET name = $2, gender = $3, sort_order = $4, is_active = $5, preview_url = $6,
		    parent_id = $7, section = $8, screen_key = $9, media_kind = $10, prompt_gender = $11,
		    prompts_screen_key = $12, photo_screen_key = $13
		WHERE id = $1`,
		id, in.Name, in.Gender, in.SortOrder, in.IsActive, in.PreviewURL,
		in.ParentID, in.Section, in.ScreenKey, in.MediaKind, in.PromptGender,
		in.PromptsScreenKey, in.PhotoScreenKey)
	if err != nil {
		return err
	}

	// Раздел переехал вместе с родителем — тянем его на всё поддерево.
	_, err = r.db.Exec(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM categories WHERE parent_id = $1
			UNION ALL
			SELECT c.id FROM categories AS c JOIN subtree AS s ON c.parent_id = s.id
		)
		UPDATE categories SET section = $2 WHERE id IN (SELECT id FROM subtree)`,
		id, in.Section)
	return err
}

func (r *CategoryRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}

func categoryScanTargets(c *Category) []any {
	return []any{
		&c.ID, &c.Name, &c.GroupID, &c.PreviewURL, &c.Gender, &c.SortOrder, &c.IsActive,
		&c.ParentID, &c.Section, &c.ScreenKey, &c.PromptsScreenKey, &c.PhotoScreenKey,
		&c.MediaKind, &c.PromptGender,
	}
}

func scanCategories(rows pgx.Rows) ([]*Category, error) {
	var cats []*Category
	for rows.Next() {
		c := &Category{}
		if err := rows.Scan(categoryScanTargets(c)...); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

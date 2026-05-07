package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Button struct {
	Label   string `json:"label"`
	Payload string `json:"payload"`
}

type Message struct {
	Key          string
	Text         string
	ImageURL     *string
	VkAttachment *string
	Buttons      []Button
	UpdatedAt    time.Time
}

type MessageRepo struct {
	db *pgxpool.Pool
}

func NewMessageRepo(db *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{db: db}
}

func (r *MessageRepo) Get(ctx context.Context, key string) (*Message, error) {
	m := &Message{}
	var buttonsJSON *[]byte
	err := r.db.QueryRow(ctx, `
		SELECT key, text, image_url, vk_attachment, buttons::text, updated_at
		FROM messages WHERE key = $1`, key).
		Scan(&m.Key, &m.Text, &m.ImageURL, &m.VkAttachment, &buttonsJSON, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		return &Message{Key: key, Text: key}, nil
	}
	if err != nil {
		return nil, err
	}
	if buttonsJSON != nil {
		_ = json.Unmarshal(*buttonsJSON, &m.Buttons)
	}
	return m, nil
}

func (r *MessageRepo) List(ctx context.Context) ([]*Message, error) {
	rows, err := r.db.Query(ctx, `SELECT key, text, image_url, vk_attachment, buttons::text, updated_at FROM messages ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		var buttonsJSON *[]byte
		if err := rows.Scan(&m.Key, &m.Text, &m.ImageURL, &m.VkAttachment, &buttonsJSON, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if buttonsJSON != nil {
			_ = json.Unmarshal(*buttonsJSON, &m.Buttons)
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *MessageRepo) Upsert(ctx context.Context, key, text string, imageURL *string, buttons []Button) error {
	var buttonsJSON []byte
	var err error
	if len(buttons) > 0 {
		buttonsJSON, err = json.Marshal(buttons)
		if err != nil {
			return err
		}
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (key, text, image_url, buttons, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (key) DO UPDATE SET
			text = $2,
			image_url = $3,
			buttons = $4,
			vk_attachment = CASE
				WHEN messages.image_url IS DISTINCT FROM $3 THEN NULL
				ELSE messages.vk_attachment
			END,
			updated_at = now()`,
		key, text, imageURL, buttonsJSON)
	return err
}

func (r *MessageRepo) SetVkAttachment(ctx context.Context, key, attachment string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE messages SET vk_attachment = $2 WHERE key = $1`,
		key, attachment)
	return err
}

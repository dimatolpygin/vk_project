package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vk_neuro_bot/internal/content"
)

type Message struct {
	Key          string
	Text         string
	ImageURL     *string
	VkAttachment *string
	Keyboard     content.Keyboard
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
	var legacyButtonsJSON *[]byte
	var keyboardJSON *[]byte
	err := r.db.QueryRow(ctx, `
		SELECT key, text, image_url, vk_attachment, buttons::text, keyboard::text, updated_at
		FROM messages WHERE key = $1`, key).
		Scan(&m.Key, &m.Text, &m.ImageURL, &m.VkAttachment, &legacyButtonsJSON, &keyboardJSON, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		if def, ok := content.Definition(key); ok {
			return &Message{Key: key, Text: def.DefaultText, Keyboard: def.Keyboard}, nil
		}
		return &Message{Key: key, Text: key}, nil
	}
	if err != nil {
		return nil, err
	}
	m.Keyboard = hydrateKeyboard(m.Key, legacyButtonsJSON, keyboardJSON)
	return m, nil
}

func (r *MessageRepo) List(ctx context.Context) ([]*Message, error) {
	rows, err := r.db.Query(ctx, `SELECT key, text, image_url, vk_attachment, buttons::text, keyboard::text, updated_at FROM messages ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		var legacyButtonsJSON *[]byte
		var keyboardJSON *[]byte
		if err := rows.Scan(&m.Key, &m.Text, &m.ImageURL, &m.VkAttachment, &legacyButtonsJSON, &keyboardJSON, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Keyboard = hydrateKeyboard(m.Key, legacyButtonsJSON, keyboardJSON)
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *MessageRepo) Upsert(ctx context.Context, key, text string, imageURL *string, keyboard content.Keyboard) error {
	keyboardJSON, err := json.Marshal(keyboard)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (key, text, image_url, keyboard, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (key) DO UPDATE SET
			text = $2,
			image_url = $3,
			keyboard = $4,
			vk_attachment = CASE
				WHEN messages.image_url IS DISTINCT FROM $3 THEN NULL
				ELSE messages.vk_attachment
			END,
			updated_at = now()`,
		key, text, imageURL, keyboardJSON)
	return err
}

func (r *MessageRepo) SetVkAttachment(ctx context.Context, key, attachment string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE messages SET vk_attachment = $2 WHERE key = $1`,
		key, attachment)
	return err
}

func (r *MessageRepo) EnsureDefaults(ctx context.Context) error {
	for _, def := range content.DefaultScreens() {
		keyboardJSON, err := json.Marshal(def.Keyboard)
		if err != nil {
			return err
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO messages (key, text, keyboard, updated_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (key) DO NOTHING`,
			def.Key, def.DefaultText, keyboardJSON); err != nil {
			return err
		}
	}

	rows, err := r.db.Query(ctx, `SELECT key, text, buttons::text, keyboard::text FROM messages`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var text string
		var legacyButtonsJSON *[]byte
		var keyboardJSON *[]byte
		if err := rows.Scan(&key, &text, &legacyButtonsJSON, &keyboardJSON); err != nil {
			return err
		}

		current := content.Keyboard{}
		if keyboardJSON != nil {
			_ = json.Unmarshal(*keyboardJSON, &current)
		}
		canonical := hydrateKeyboard(key, legacyButtonsJSON, keyboardJSON)
		upgradedText := upgradedDefaultMessageText(key, text)
		keyboardChanged := !reflect.DeepEqual(current, canonical)
		textChanged := upgradedText != text

		if !keyboardChanged && !textChanged {
			continue
		}

		switch {
		case keyboardChanged && textChanged:
			payload, err := json.Marshal(canonical)
			if err != nil {
				return err
			}
			if _, err := r.db.Exec(ctx, `UPDATE messages SET text = $2, keyboard = $3, updated_at = now() WHERE key = $1`, key, upgradedText, payload); err != nil {
				return err
			}
		case keyboardChanged:
			payload, err := json.Marshal(canonical)
			if err != nil {
				return err
			}
			if _, err := r.db.Exec(ctx, `UPDATE messages SET keyboard = $2, updated_at = now() WHERE key = $1`, key, payload); err != nil {
				return err
			}
		case textChanged:
			if _, err := r.db.Exec(ctx, `UPDATE messages SET text = $2, updated_at = now() WHERE key = $1`, key, upgradedText); err != nil {
				return err
			}
		}
	}

	return rows.Err()
}

func (r *MessageRepo) WaitForKeyboardSchema(ctx context.Context, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		ready, err := r.hasKeyboardSchema(deadlineCtx)
		if err == nil && ready {
			return nil
		}
		if err != nil {
			lastErr = err
		}

		select {
		case <-deadlineCtx.Done():
			if lastErr != nil {
				return fmt.Errorf("wait for messages.keyboard schema: %w", lastErr)
			}
			return fmt.Errorf("wait for messages.keyboard schema: %w", deadlineCtx.Err())
		case <-ticker.C:
		}
	}
}

func (r *MessageRepo) hasKeyboardSchema(ctx context.Context) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = 'messages'
				AND column_name = 'keyboard'
		)`).
		Scan(&exists)
	return exists, err
}

func hydrateKeyboard(key string, legacyButtonsJSON, keyboardJSON *[]byte) content.Keyboard {
	if keyboardJSON != nil && len(*keyboardJSON) > 0 {
		var kb content.Keyboard
		if err := json.Unmarshal(*keyboardJSON, &kb); err == nil {
			return content.CanonicalizeKeyboard(key, kb)
		}
	}
	if legacyButtonsJSON != nil && len(*legacyButtonsJSON) > 0 {
		var legacy []content.LegacyButton
		if err := json.Unmarshal(*legacyButtonsJSON, &legacy); err == nil {
			return content.BuildLegacyKeyboard(key, legacy)
		}
	}
	if def, ok := content.Definition(key); ok {
		return def.Keyboard
	}
	return content.Keyboard{Inline: true}
}

func upgradedDefaultMessageText(key, text string) string {
	legacyTexts, ok := legacyDefaultMessageTexts[key]
	if !ok {
		return text
	}
	matchesLegacy := false
	for _, legacyText := range legacyTexts {
		if text == legacyText {
			matchesLegacy = true
			break
		}
	}
	if !matchesLegacy {
		return text
	}

	def, ok := content.Definition(key)
	if !ok {
		return text
	}
	return def.DefaultText
}

var legacyDefaultMessageTexts = map[string][]string{
	"custom_prompt_intro": {
		"✍️ Опиши желаемый стиль фотосессии своими словами.\n\nПример: «деловой костюм на фоне небоскрёбов, профессиональное освещение»\n\nВведи описание:",
	},
	"edit_photo_intro": {
		"✏️ Изменить фото\n\nОтправь от 1 до 6 фото одним сообщением, которые хочешь изменить, и затем опиши что нужно сделать:",
	},
	"payment_success": {
		"🎉 Оплата прошла успешно!\n\nДобро пожаловать в мир нейрофотосессий! Твои генерации зачислены.",
	},
	"saved_photo_upload_prompt": {
		"📤 Отправьте фото — оно сохранится и будет использоваться по умолчанию:",
	},
	"settings_overview": {
		"⚙️ Настройки\n\n🎯 Баланс генераций: {{.TotalGens}}\n🤖 Модель: {{.ModelName}}\n🔧 Качество: {{.Resolution}}\n📐 Формат: {{.AspectRatio}}",
		"⚙️ Настройки\n\n🎯 Баланс генераций: {{.TotalGens}}\n🤖 Модель: {{.ModelName}}\n🔧 Качество: {{.Resolution}}\n📐 Формат: {{.AspectRatio}}\n\n🎁 Приглашено рефералов: {{.ReferralCount}}{{if .ReferralLink}}\n🔗 Твоя ссылка:\n{{.ReferralLink}}{{end}}",
	},
}

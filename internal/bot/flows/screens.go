package flows

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

type ScreenOptions struct {
	Data                map[string]any
	SelectedValue       string
	ToggleOn            bool
	Links               map[string]string
	PrefixRows          [][]KbBtn
	ImageOverride       *string
	AttachmentsOverride []string // готовые VK attachment strings (без повторной загрузки)
}

func sendScreen(ctx context.Context, d *Deps, vkID int64, key string, opts ScreenOptions) error {
	var (
		msg *repository.Message
		err error
	)
	if d.MsgRepo != nil {
		msg, err = d.MsgRepo.Get(ctx, key)
		if err != nil {
			return err
		}
	} else if def, ok := content.Definition(key); ok {
		msg = &repository.Message{
			Key:      key,
			Text:     def.DefaultText,
			Keyboard: def.Keyboard,
		}
	} else {
		msg = &repository.Message{
			Key:      key,
			Text:     key,
			Keyboard: content.Keyboard{Inline: true},
		}
	}

	text, err := content.RenderText(msg.Text, opts.Data)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("не удалось отрендерить шаблон текста")
	}

	imageURL := msg.ImageURL
	cacheKey := key
	var attachments []string

	if len(opts.AttachmentsOverride) > 0 {
		attachments = opts.AttachmentsOverride
		cacheKey = ""
	} else if opts.ImageOverride != nil {
		imageURL = opts.ImageOverride
		cacheKey = ""
	}

	return d.Sender.SendScreen(ctx, vkID, &ScreenMessage{
		Key:         key,
		Text:        text,
		ImageURL:    imageURL,
		Attachments: attachments,
		Keyboard: RenderContentKeyboardWithRows(msg.Keyboard, opts.PrefixRows, KeyboardRenderOptions{
			SelectedValue: opts.SelectedValue,
			ToggleOn:      opts.ToggleOn,
			Links:         opts.Links,
		}),
		CacheKey: cacheKey,
	})
}

// TariffRows строит ряды кнопок покупки тарифов — используется и в экране тарифов,
// и в догоняющем напоминании из воркера.
func TariffRows(tariffs []*repository.Tariff) [][]KbBtn {
	return tariffRows(tariffs)
}

func tariffRows(tariffs []*repository.Tariff) [][]KbBtn {
	rows := make([][]KbBtn, 0, len(tariffs))
	for _, tariff := range tariffs {
		payload, _ := jsonMarshal(map[string]any{"type": "buy_tariff", "tariff_id": tariff.ID})
		label := fmt.Sprintf("💳 %s — %.0f₽ (%d ген.)", tariff.Name, tariff.Price, tariff.GensCount)
		rows = append(rows, []KbBtn{{
			Action: KbAction{Type: "callback", Label: label, Payload: payload},
			Color:  "primary",
		}})
	}
	return rows
}

func categoryButtons(categories []*repository.Category) []KbBtn {
	buttons := make([]KbBtn, 0, len(categories))
	for _, category := range categories {
		payload, _ := jsonMarshal(map[string]any{"type": "select_category", "category_id": category.ID})
		buttons = append(buttons, KbBtn{
			Action: KbAction{Type: "callback", Label: category.Name, Payload: payload},
			Color:  "primary",
		})
	}
	return buttons
}

func promptButtons(prompts []*repository.Prompt) []KbBtn {
	buttons := make([]KbBtn, 0, len(prompts))
	for _, prompt := range prompts {
		payload, _ := jsonMarshal(map[string]any{"type": "select_prompt", "prompt_id": prompt.ID})
		buttons = append(buttons, KbBtn{
			Action: KbAction{Type: "callback", Label: prompt.Name, Payload: payload},
			Color:  "primary",
		})
	}
	return buttons
}

const (
	paginatedColumns     = 2
	paginatedRowsPerPage = 2
)

func buildPaginatedRows(buttons []KbBtn, page int, pagerType string, pagerExtra map[string]any) ([][]KbBtn, int, int) {
	perPage := paginatedColumns * paginatedRowsPerPage
	if perPage <= 0 {
		perPage = 1
	}

	totalPages := 1
	if len(buttons) > 0 {
		totalPages = (len(buttons) + perPage - 1) / perPage
	}

	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * perPage
	end := start + perPage
	if end > len(buttons) {
		end = len(buttons)
	}

	rows := make([][]KbBtn, 0, paginatedRowsPerPage+1)
	for i := start; i < end; i += paginatedColumns {
		rowEnd := i + paginatedColumns
		if rowEnd > end {
			rowEnd = end
		}
		row := make([]KbBtn, rowEnd-i)
		copy(row, buttons[i:rowEnd])
		rows = append(rows, row)
	}

	if pagerRow := buildPagerRow(page, totalPages, pagerType, pagerExtra); len(pagerRow) > 0 {
		rows = append(rows, pagerRow)
	}

	return rows, page, totalPages
}

func buildPagerRow(page, totalPages int, pagerType string, pagerExtra map[string]any) []KbBtn {
	if totalPages <= 1 || pagerType == "" {
		return nil
	}

	row := make([]KbBtn, 0, 2)
	if page > 1 {
		row = append(row, pagerButton("⬅️ Назад", pagerType, page-1, pagerExtra))
	}
	if page < totalPages {
		row = append(row, pagerButton("Вперёд ➡️", pagerType, page+1, pagerExtra))
	}
	return row
}

func pagerButton(label, pagerType string, page int, pagerExtra map[string]any) KbBtn {
	payloadData := map[string]any{
		"type": pagerType,
		"page": page,
	}
	for key, value := range pagerExtra {
		payloadData[key] = value
	}
	payload, _ := jsonMarshal(payloadData)
	return KbBtn{
		Action: KbAction{Type: "callback", Label: label, Payload: payload},
		Color:  "secondary",
	}
}

func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

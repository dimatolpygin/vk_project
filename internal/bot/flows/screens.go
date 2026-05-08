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
	Data          map[string]any
	SelectedValue string
	ToggleOn      bool
	Links         map[string]string
	PrefixRows    [][]KbBtn
	ImageOverride *string
}

func sendScreen(ctx context.Context, d *Deps, vkID int64, key string, opts ScreenOptions) error {
	msg, err := d.MsgRepo.Get(ctx, key)
	if err != nil {
		return err
	}

	text, err := content.RenderText(msg.Text, opts.Data)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("не удалось отрендерить шаблон текста")
	}

	imageURL := msg.ImageURL
	cacheKey := key
	if opts.ImageOverride != nil {
		imageURL = opts.ImageOverride
		cacheKey = ""
	}

	return d.Sender.SendScreen(ctx, vkID, &ScreenMessage{
		Key:      key,
		Text:     text,
		ImageURL: imageURL,
		Keyboard: RenderContentKeyboardWithRows(msg.Keyboard, opts.PrefixRows, KeyboardRenderOptions{
			SelectedValue: opts.SelectedValue,
			ToggleOn:      opts.ToggleOn,
			Links:         opts.Links,
		}),
		CacheKey: cacheKey,
	})
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

func categoryRows(categories []*repository.Category) [][]KbBtn {
	rows := make([][]KbBtn, 0, len(categories))
	for _, category := range categories {
		payload, _ := jsonMarshal(map[string]any{"type": "select_category", "category_id": category.ID})
		rows = append(rows, []KbBtn{{
			Action: KbAction{Type: "callback", Label: category.Name, Payload: payload},
			Color:  "primary",
		}})
	}
	return rows
}

func promptRows(prompts []*repository.Prompt) [][]KbBtn {
	rows := make([][]KbBtn, 0, len(prompts))
	for _, prompt := range prompts {
		payload, _ := jsonMarshal(map[string]any{"type": "select_prompt", "prompt_id": prompt.ID})
		rows = append(rows, []KbBtn{{
			Action: KbAction{Type: "callback", Label: prompt.Name, Payload: payload},
			Color:  "primary",
		}})
	}
	return rows
}

func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

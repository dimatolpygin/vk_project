package flows

import (
	"encoding/json"
	"fmt"

	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
)

type Keyboard struct {
	OneTime bool      `json:"one_time,omitempty"`
	Buttons [][]KbBtn `json:"buttons"`
	Inline  bool      `json:"inline"`
}

type KbBtn struct {
	Action KbAction `json:"action"`
	Color  string   `json:"color,omitempty"`
}

type KbAction struct {
	Type    string `json:"type"`
	Label   string `json:"label,omitempty"`
	Payload string `json:"payload,omitempty"`
	Link    string `json:"link,omitempty"`
}

type KeyboardRenderOptions struct {
	SelectedValue string
	ToggleOn      bool
	Links         map[string]string
}

func kbJSON(kb *Keyboard) string {
	if kb == nil {
		return ""
	}
	b, _ := json.Marshal(kb)
	return string(b)
}

func cbPayload(t string) string {
	b, _ := json.Marshal(map[string]string{"type": t})
	return string(b)
}

func btnBack() KbBtn {
	return KbBtn{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}
}

func KbBack() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{{btnBack()}}})
}

func KbMainMenu() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: "🖼 Готовые промты", Payload: cbPayload("ready_prompts")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "✍️ Свой промт", Payload: cbPayload("custom_prompt")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "✏️ Изменить фото", Payload: cbPayload("edit_photo")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "👫 Парное фото", Payload: cbPayload("couple")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "💾 Запомнить фото", Payload: cbPayload("saved_photo")}, Color: "secondary"}},
		{{Action: KbAction{Type: "callback", Label: "⚙️ Настройки", Payload: cbPayload("settings")}, Color: "secondary"}},
	}})
}

func KbWelcome() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: "✨ Попробовать бесплатно", Payload: cbPayload("free_gen")}, Color: "positive"}},
		{{Action: KbAction{Type: "callback", Label: "🎁 Реферальная программа", Payload: cbPayload("referral")}, Color: "secondary"}},
	}})
}

func KbGender() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{{
		{Action: KbAction{Type: "callback", Label: "👨 Мужской", Payload: cbPayload("gender_male")}, Color: "primary"},
		{Action: KbAction{Type: "callback", Label: "👩 Женский", Payload: cbPayload("gender_female")}, Color: "primary"},
	}}})
}

func KbSubscribeCTA(groupURL string) string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "open_link", Label: "📌 Подписаться на группу", Link: groupURL}}},
		{{Action: KbAction{Type: "callback", Label: "✅ Я подписался", Payload: cbPayload("check_sub")}, Color: "positive"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}},
	}})
}

func KbAfterGen() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: "✨ Ещё одно фото", Payload: cbPayload("free_gen")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "🚀 Активировать все функции", Payload: cbPayload("tariffs")}, Color: "positive"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}},
	}})
}

func KbAfterGenPaid(photoURL string) string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "open_link", Label: "⬇️ Скачать фото", Link: photoURL}}},
		{{Action: KbAction{Type: "callback", Label: "✏️ Изменить фото", Payload: cbPayload("edit_result")}, Color: "secondary"}},
		{{Action: KbAction{Type: "callback", Label: "✨ Сгенерировать еще", Payload: cbPayload("gen_again")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}},
	}})
}

func KbTariffs(tariffs []*repository.Tariff) string {
	kb := &Keyboard{Inline: true}
	for _, t := range tariffs {
		payload, _ := json.Marshal(map[string]any{"type": "buy_tariff", "tariff_id": t.ID})
		label := fmt.Sprintf("💳 %s — %.0f₽ (%d ген.)", t.Name, t.Price, t.GensCount)
		kb.Buttons = append(kb.Buttons, []KbBtn{{
			Action: KbAction{Type: "callback", Label: label, Payload: string(payload)},
			Color:  "primary",
		}})
	}
	kb.Buttons = append(kb.Buttons, []KbBtn{btnBack()})
	return kbJSON(kb)
}

func KbCategories(cats []*repository.Category) string {
	kb := &Keyboard{Inline: true}
	for _, c := range cats {
		payload, _ := json.Marshal(map[string]any{"type": "select_category", "category_id": c.ID})
		kb.Buttons = append(kb.Buttons, []KbBtn{{
			Action: KbAction{Type: "callback", Label: c.Name, Payload: string(payload)},
			Color:  "primary",
		}})
	}
	kb.Buttons = append(kb.Buttons, []KbBtn{btnBack()})
	return kbJSON(kb)
}

func KbPrompts(prompts []*repository.Prompt) string {
	kb := &Keyboard{Inline: true}
	for _, p := range prompts {
		payload, _ := json.Marshal(map[string]any{"type": "select_prompt", "prompt_id": p.ID})
		kb.Buttons = append(kb.Buttons, []KbBtn{{
			Action: KbAction{Type: "callback", Label: p.Name, Payload: string(payload)},
			Color:  "primary",
		}})
	}
	kb.Buttons = append(kb.Buttons, []KbBtn{btnBack()})
	return kbJSON(kb)
}

func KbSettings() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: "🔧 Качество", Payload: cbPayload("quality")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "📐 Формат", Payload: cbPayload("format")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "💳 Баланс", Payload: cbPayload("balance")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "🤖 Модель", Payload: cbPayload("model")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}},
	}})
}

func KbModel(current string) string {
	mark := func(id string) string {
		if current == id {
			return "✅ "
		}
		return ""
	}
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: mark("google/nano-banana-pro") + "Nano Banana Pro", Payload: cbPayload("model_nbp")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: mark("google/nano-banana-2") + "Nano Banana 2", Payload: cbPayload("model_nb2")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: mark("openai/gpt-image-2") + "GPT Image 2", Payload: cbPayload("model_gpt2")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("settings")}, Color: "secondary"}},
	}})
}

func KbQuality(current string) string {
	mark := func(v string) string {
		if current == v {
			return "✅ "
		}
		return ""
	}
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: mark("1k") + "Стандарт (1k)", Payload: cbPayload("quality_1k")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: mark("2k") + "HD (2k)", Payload: cbPayload("quality_2k")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: mark("4k") + "Ультра (4k)", Payload: cbPayload("quality_4k")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("settings")}, Color: "secondary"}},
	}})
}

func KbAspectRatio(current string) string {
	mark := func(v string) string {
		if current == v {
			return "✅ "
		}
		return ""
	}
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: mark("1:1") + "1:1 (Квадрат)", Payload: cbPayload("ar_1_1")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: mark("9:16") + "9:16 (Портрет)", Payload: cbPayload("ar_9_16")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: mark("16:9") + "16:9 (Пейзаж)", Payload: cbPayload("ar_16_9")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("settings")}, Color: "secondary"}},
	}})
}

func KbBottomMenu() string {
	if def, ok := content.Definition("bottom_menu"); ok {
		return RenderContentKeyboard(def.Keyboard, KeyboardRenderOptions{})
	}
	return ""
}

func KbSavedPhotoMenu(hasSavedPhoto, useInPrompts bool) string {
	kb := &Keyboard{Inline: true}
	if !hasSavedPhoto {
		kb.Buttons = append(kb.Buttons, []KbBtn{{
			Action: KbAction{Type: "callback", Label: "📤 Загрузить фото", Payload: cbPayload("saved_photo_upload")},
			Color:  "primary",
		}})
	} else {
		toggleLabel := "▶️ Включить в готовых промтах"
		if useInPrompts {
			toggleLabel = "⏸ Выключить в готовых промтах"
		}
		kb.Buttons = append(kb.Buttons,
			[]KbBtn{{Action: KbAction{Type: "callback", Label: "✏️ Заменить фото", Payload: cbPayload("saved_photo_upload")}, Color: "secondary"}},
			[]KbBtn{{Action: KbAction{Type: "callback", Label: toggleLabel, Payload: cbPayload("toggle_saved_photo")}, Color: "primary"}},
		)
	}
	kb.Buttons = append(kb.Buttons, []KbBtn{btnBack()})
	return kbJSON(kb)
}

func RenderContentKeyboard(cfg content.Keyboard, opts KeyboardRenderOptions) string {
	return renderContentKeyboardWithRows(cfg, nil, opts)
}

func RenderContentKeyboardWithRows(cfg content.Keyboard, prefixRows [][]KbBtn, opts KeyboardRenderOptions) string {
	return renderContentKeyboardWithRows(cfg, prefixRows, opts)
}

func renderContentKeyboardWithRows(cfg content.Keyboard, prefixRows [][]KbBtn, opts KeyboardRenderOptions) string {
	rows := make([][]KbBtn, 0, len(prefixRows)+len(cfg.Items))
	rows = append(rows, prefixRows...)
	rows = append(rows, contentRows(cfg, opts)...)
	return kbJSON(&Keyboard{Inline: cfg.Inline, OneTime: cfg.OneTime, Buttons: rows})
}

func contentRows(cfg content.Keyboard, opts KeyboardRenderOptions) [][]KbBtn {
	if len(cfg.Items) == 0 {
		return nil
	}

	var rows [][]KbBtn
	currentRow := -1
	var current []KbBtn
	for _, item := range cfg.Items {
		if !item.Visible {
			continue
		}
		btn, ok := contentButton(item, opts)
		if !ok {
			continue
		}
		if currentRow == -1 {
			currentRow = item.Row
		}
		if item.Row != currentRow {
			if len(current) > 0 {
				rows = append(rows, current)
			}
			currentRow = item.Row
			current = nil
		}
		current = append(current, btn)
	}
	if len(current) > 0 {
		rows = append(rows, current)
	}
	return rows
}

func contentButton(item content.Button, opts KeyboardRenderOptions) (KbBtn, bool) {
	switch item.Kind {
	case content.ButtonKindText:
		return KbBtn{
			Action: KbAction{Type: "text", Label: item.Label, Payload: cbPayload(item.ActionKey)},
			Color:  item.Color,
		}, true
	case content.ButtonKindOpenLink:
		link := opts.Links[item.LinkBinding]
		if link == "" {
			return KbBtn{}, false
		}
		return KbBtn{
			Action: KbAction{Type: "open_link", Label: item.Label, Link: link},
			Color:  item.Color,
		}, true
	case content.ButtonKindSelect:
		label := item.Label
		if item.Value != "" && item.Value == opts.SelectedValue {
			label = "✅ " + label
		}
		return KbBtn{
			Action: KbAction{Type: "callback", Label: label, Payload: cbPayload(item.ActionKey)},
			Color:  item.Color,
		}, true
	case content.ButtonKindToggle:
		label := item.Label
		if opts.ToggleOn && item.LabelOn != "" {
			label = item.LabelOn
		}
		return KbBtn{
			Action: KbAction{Type: "callback", Label: label, Payload: cbPayload(item.ActionKey)},
			Color:  item.Color,
		}, true
	default:
		return KbBtn{
			Action: KbAction{Type: "callback", Label: item.Label, Payload: cbPayload(item.ActionKey)},
			Color:  item.Color,
		}, true
	}
}

package flows

import (
	"encoding/json"
	"fmt"

	"vk_neuro_bot/internal/repository"
)

type Keyboard struct {
	OneTime bool       `json:"one_time,omitempty"`
	Buttons [][]KbBtn  `json:"buttons"`
	Inline  bool       `json:"inline"`
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

func KbFromMsg(buttons []repository.Button) string {
	kb := &Keyboard{Inline: true}
	for _, b := range buttons {
		p, _ := json.Marshal(map[string]string{"type": b.Payload})
		kb.Buttons = append(kb.Buttons, []KbBtn{{
			Action: KbAction{Type: "callback", Label: b.Label, Payload: string(p)},
			Color:  "primary",
		}})
	}
	return kbJSON(kb)
}

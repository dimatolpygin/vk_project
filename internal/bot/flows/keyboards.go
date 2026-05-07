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

func KbAfterGenPaid(photoURL string) string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "open_link", Label: "⬇️ Скачать фото", Link: photoURL}}},
		{{Action: KbAction{Type: "callback", Label: "✨ Сгенерировать еще", Payload: cbPayload("free_gen")}, Color: "primary"}},
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
	return kbJSON(&Keyboard{Inline: false, OneTime: false, Buttons: [][]KbBtn{
		{
			{Action: KbAction{Type: "callback", Label: "🏠 Главное меню", Payload: cbPayload("main_menu")}, Color: "secondary"},
			{Action: KbAction{Type: "callback", Label: "💎 Купить генерации", Payload: cbPayload("buy_gens")}, Color: "primary"},
		},
		{
			{Action: KbAction{Type: "callback", Label: "⚙️ Настройки", Payload: cbPayload("settings")}, Color: "secondary"},
			{Action: KbAction{Type: "callback", Label: "🆘 Техподдержка", Payload: cbPayload("support")}, Color: "secondary"},
			{Action: KbAction{Type: "callback", Label: "🖼 Примеры работ", Payload: cbPayload("examples")}, Color: "secondary"},
		},
	}})
}

func KbCoupleMenu() string {
	return kbJSON(&Keyboard{Inline: true, Buttons: [][]KbBtn{
		{{Action: KbAction{Type: "callback", Label: "👫 Пара", Payload: cbPayload("couple_pair")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "👨‍👩‍👧 Семейное фото", Payload: cbPayload("couple_family")}, Color: "primary"}},
		{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("back")}, Color: "secondary"}},
	}})
}

func KbCouplePrompts(coupleType string) string {
	kb := &Keyboard{Inline: true}
	kb.Buttons = append(kb.Buttons, []KbBtn{
		{Action: KbAction{Type: "callback", Label: "🌹 Романтическая прогулка", Payload: cbPayload("couple_romantic")}, Color: "primary"},
	})
	if coupleType == "pair" {
		kb.Buttons = append(kb.Buttons, []KbBtn{
			{Action: KbAction{Type: "callback", Label: "💼 Деловой портрет", Payload: cbPayload("couple_business")}, Color: "primary"},
		})
	}
	if coupleType == "family" {
		kb.Buttons = append(kb.Buttons, []KbBtn{
			{Action: KbAction{Type: "callback", Label: "🏠 Семейный уют", Payload: cbPayload("couple_family_cozy")}, Color: "primary"},
		})
	}
	kb.Buttons = append(kb.Buttons,
		[]KbBtn{{Action: KbAction{Type: "callback", Label: "🎨 Художественный стиль", Payload: cbPayload("couple_art")}, Color: "primary"}},
		[]KbBtn{{Action: KbAction{Type: "callback", Label: "◀️ Назад", Payload: cbPayload("couple_menu")}, Color: "secondary"}},
	)
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

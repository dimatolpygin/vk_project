package content

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"
)

const (
	ButtonKindCallback = "callback"
	ButtonKindText     = "text"
	ButtonKindOpenLink = "open_link"
	ButtonKindSelect   = "select"
	ButtonKindToggle   = "toggle"
)

type LegacyButton struct {
	Label   string `json:"label"`
	Payload string `json:"payload"`
}

type Keyboard struct {
	Inline  bool     `json:"inline"`
	OneTime bool     `json:"one_time,omitempty"`
	Items   []Button `json:"items"`
}

type Button struct {
	SlotID      string `json:"slot_id"`
	ActionKey   string `json:"action_key"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	LabelOn     string `json:"label_on,omitempty"`
	Row         int    `json:"row"`
	Position    int    `json:"position"`
	Visible     bool   `json:"visible"`
	Color       string `json:"color,omitempty"`
	Value       string `json:"value,omitempty"`
	LinkBinding string `json:"link_binding,omitempty"`
}

type ScreenDefinition struct {
	Key             string
	DefaultText     string
	DefaultImageURL string
	Keyboard        Keyboard
}

type byRowPosition []Button

func (b byRowPosition) Len() int      { return len(b) }
func (b byRowPosition) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byRowPosition) Less(i, j int) bool {
	if b[i].Row != b[j].Row {
		return b[i].Row < b[j].Row
	}
	if b[i].Position != b[j].Position {
		return b[i].Position < b[j].Position
	}
	return b[i].SlotID < b[j].SlotID
}

func DefaultScreens() []ScreenDefinition {
	out := make([]ScreenDefinition, 0, len(screenDefinitions))
	for _, def := range screenDefinitions {
		out = append(out, cloneDefinition(def))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func Definition(key string) (ScreenDefinition, bool) {
	def, ok := screenDefinitions[key]
	if !ok {
		return ScreenDefinition{}, false
	}
	return cloneDefinition(def), true
}

func CanonicalizeKeyboard(key string, stored Keyboard) Keyboard {
	def, ok := Definition(key)
	if !ok {
		return normalizeUnknownKeyboard(stored)
	}
	kb, err := mergeKeyboard(def.Keyboard, stored, false)
	if err != nil {
		return def.Keyboard
	}
	return kb
}

func MergeEditableKeyboard(key string, input Keyboard) (Keyboard, error) {
	def, ok := Definition(key)
	if !ok {
		return normalizeUnknownKeyboard(input), nil
	}
	return mergeKeyboard(def.Keyboard, input, true)
}

func BuildLegacyKeyboard(key string, legacy []LegacyButton) Keyboard {
	def, ok := Definition(key)
	if !ok {
		items := make([]Button, 0, len(legacy))
		for i, btn := range legacy {
			items = append(items, Button{
				SlotID:    fmt.Sprintf("legacy_%d", i),
				ActionKey: btn.Payload,
				Kind:      ButtonKindCallback,
				Label:     btn.Label,
				Row:       i,
				Position:  0,
				Visible:   true,
				Color:     "primary",
			})
		}
		return Keyboard{Inline: true, Items: items}
	}

	out := def.Keyboard
	labelsByAction := make(map[string]string, len(legacy))
	for _, btn := range legacy {
		if btn.Payload == "" {
			continue
		}
		labelsByAction[btn.Payload] = btn.Label
	}
	for i := range out.Items {
		if label, ok := labelsByAction[out.Items[i].ActionKey]; ok && label != "" {
			out.Items[i].Label = label
		}
	}
	sort.Sort(byRowPosition(out.Items))
	return out
}

func RenderText(raw string, data any) (string, error) {
	if raw == "" {
		return "", nil
	}
	tpl, err := template.New("screen").Option("missingkey=zero").Parse(raw)
	if err != nil {
		return raw, err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return raw, err
	}
	return buf.String(), nil
}

func cloneDefinition(def ScreenDefinition) ScreenDefinition {
	return ScreenDefinition{
		Key:             def.Key,
		DefaultText:     def.DefaultText,
		DefaultImageURL: def.DefaultImageURL,
		Keyboard:        cloneKeyboard(def.Keyboard),
	}
}

func cloneKeyboard(kb Keyboard) Keyboard {
	out := Keyboard{
		Inline:  kb.Inline,
		OneTime: kb.OneTime,
		Items:   make([]Button, len(kb.Items)),
	}
	copy(out.Items, kb.Items)
	return out
}

func mergeKeyboard(def Keyboard, input Keyboard, strict bool) (Keyboard, error) {
	out := cloneKeyboard(def)
	in := make(map[string]Button, len(input.Items))
	for _, item := range input.Items {
		if item.SlotID == "" {
			if strict {
				return Keyboard{}, fmt.Errorf("button slot_id is required")
			}
			continue
		}
		in[item.SlotID] = item
	}

	if strict {
		for _, item := range input.Items {
			found := false
			for _, defItem := range def.Items {
				if defItem.SlotID == item.SlotID {
					found = true
					break
				}
			}
			if !found {
				return Keyboard{}, fmt.Errorf("unknown button slot_id %q", item.SlotID)
			}
		}
	}

	for i := range out.Items {
		stored, ok := in[out.Items[i].SlotID]
		if !ok {
			if strict {
				return Keyboard{}, fmt.Errorf("missing button slot_id %q", out.Items[i].SlotID)
			}
			continue
		}
		if strict && (stored.ActionKey != out.Items[i].ActionKey || stored.Kind != out.Items[i].Kind || stored.Color != out.Items[i].Color || stored.Value != out.Items[i].Value || stored.LinkBinding != out.Items[i].LinkBinding) {
			return Keyboard{}, fmt.Errorf("button %q contains immutable field changes", out.Items[i].SlotID)
		}
		if stored.Label != "" {
			out.Items[i].Label = stored.Label
		}
		if out.Items[i].Kind == ButtonKindToggle && stored.LabelOn != "" {
			out.Items[i].LabelOn = stored.LabelOn
		}
		out.Items[i].Row = stored.Row
		out.Items[i].Position = stored.Position
		out.Items[i].Visible = stored.Visible
	}

	sort.Sort(byRowPosition(out.Items))
	return out, nil
}

func normalizeUnknownKeyboard(kb Keyboard) Keyboard {
	out := cloneKeyboard(kb)
	for i := range out.Items {
		if out.Items[i].SlotID == "" {
			out.Items[i].SlotID = fmt.Sprintf("unknown_%d", i)
		}
		if out.Items[i].Kind == "" {
			out.Items[i].Kind = ButtonKindCallback
		}
		if out.Items[i].ActionKey == "" {
			out.Items[i].ActionKey = out.Items[i].SlotID
		}
	}
	sort.Sort(byRowPosition(out.Items))
	return out
}

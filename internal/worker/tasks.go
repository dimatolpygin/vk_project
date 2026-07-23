package worker

import "encoding/json"

const TaskGenerate = "generation:process"
const TaskBroadcastProcess = "broadcast:process"
const TaskPaymentReminder = "payment:reminder"

type GeneratePayload struct {
	GenerationID int64    `json:"generation_id"`
	UserVKID     int64    `json:"user_vk_id"`
	Model        string   `json:"model"`
	Images       []string `json:"images"`
	Prompt       string   `json:"prompt"`
	Resolution   string   `json:"resolution"`
	AspectRatio  string   `json:"aspect_ratio,omitempty"`
	OutputFormat string   `json:"output_format"`
}

func (p GeneratePayload) Bytes() ([]byte, error) {
	return json.Marshal(p)
}

func ParseGeneratePayload(data []byte) (*GeneratePayload, error) {
	var p GeneratePayload
	return &p, json.Unmarshal(data, &p)
}

type BroadcastPayload struct {
	BroadcastID int64 `json:"broadcast_id"`
	BatchSize   int   `json:"batch_size,omitempty"`
}

func (p BroadcastPayload) Bytes() ([]byte, error) {
	return json.Marshal(p)
}

func ParseBroadcastPayload(data []byte) (*BroadcastPayload, error) {
	var p BroadcastPayload
	return &p, json.Unmarshal(data, &p)
}

// PaymentReminderPayload — догоняющее сообщение пользователю, который открыл
// экран тарифов и не оплатил.
type PaymentReminderPayload struct {
	UserVKID int64 `json:"user_vk_id"`
}

func (p PaymentReminderPayload) Bytes() ([]byte, error) {
	return json.Marshal(p)
}

func ParsePaymentReminderPayload(data []byte) (*PaymentReminderPayload, error) {
	var p PaymentReminderPayload
	return &p, json.Unmarshal(data, &p)
}

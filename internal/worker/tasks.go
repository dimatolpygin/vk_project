package worker

import "encoding/json"

const TaskGenerate = "generation:process"
const TaskGenerateVideo = "generation:video"
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

// GenerateVideoPayload — задача из двух звеньев: сначала фото-модель собирает
// сцену по Prompt из фото пользователя, потом сцена уходит в видео-модель
// с VideoPrompt. Параметры видео в payload не едут: они зафиксированы в коде.
type GenerateVideoPayload struct {
	GenerationID int64    `json:"generation_id"`
	UserVKID     int64    `json:"user_vk_id"`
	PhotoModel   string   `json:"photo_model"`
	Images       []string `json:"images"`
	Prompt       string   `json:"prompt"`
	VideoPrompt  string   `json:"video_prompt"`
	Resolution   string   `json:"resolution"`
	OutputFormat string   `json:"output_format"`
	PromptName   string   `json:"prompt_name,omitempty"`
	CostGens     int      `json:"cost_gens,omitempty"`
}

func (p GenerateVideoPayload) Bytes() ([]byte, error) {
	return json.Marshal(p)
}

func ParseGenerateVideoPayload(data []byte) (*GenerateVideoPayload, error) {
	var p GenerateVideoPayload
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

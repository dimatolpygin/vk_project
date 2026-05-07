package worker

import "encoding/json"

const TaskGenerate = "generation:process"

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

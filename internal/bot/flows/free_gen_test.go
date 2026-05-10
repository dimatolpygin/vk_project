package flows

import (
	"context"
	"errors"
	"testing"

	"vk_neuro_bot/internal/repository"
)

type fakeMessageReader struct {
	msg *repository.Message
	err error
}

func (f *fakeMessageReader) Get(context.Context, string) (*repository.Message, error) {
	return f.msg, f.err
}

func TestBuildDefaultPromptUsesEditableMessageTemplate(t *testing.T) {
	deps := &Deps{
		MsgRepo: &fakeMessageReader{
			msg: &repository.Message{
				Key:  freeGenerationPromptMessageKey,
				Text: "cinematic portrait of a {{.GenderLabel}}, soft rim light",
			},
		},
	}

	got := buildDefaultPrompt(context.Background(), deps, "male", "free")

	if got != "cinematic portrait of a man, soft rim light" {
		t.Fatalf("unexpected rendered prompt: %q", got)
	}
}

func TestBuildDefaultPromptFallsBackOnBrokenTemplate(t *testing.T) {
	deps := &Deps{
		MsgRepo: &fakeMessageReader{
			msg: &repository.Message{
				Key:  freeGenerationPromptMessageKey,
				Text: "{{.Broken",
			},
		},
	}

	got := buildDefaultPrompt(context.Background(), deps, "female", "free")

	if got != "professional portrait photo of a woman, studio lighting, high quality, photorealistic" {
		t.Fatalf("expected default prompt fallback, got %q", got)
	}
}

func TestBuildDefaultPromptFallsBackOnMessageError(t *testing.T) {
	deps := &Deps{
		MsgRepo: &fakeMessageReader{err: errors.New("db unavailable")},
	}

	got := buildDefaultPrompt(context.Background(), deps, "male", "free")

	if got != "professional portrait photo of a man, studio lighting, high quality, photorealistic" {
		t.Fatalf("expected default prompt fallback, got %q", got)
	}
}

func TestBuildDefaultPromptKeepsCouplePromptsHardcoded(t *testing.T) {
	got := buildDefaultPrompt(context.Background(), &Deps{}, "female", "couple_family")

	if got != "family portrait, warm atmosphere, professional photo, studio lighting, high quality" {
		t.Fatalf("unexpected couple prompt: %q", got)
	}
}

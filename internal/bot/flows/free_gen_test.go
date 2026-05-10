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

func TestNormalizeGenerationInputPhotosLimitsAndDedupes(t *testing.T) {
	got := normalizeGenerationInputPhotos([]string{
		"",
		" https://example.com/1.png ",
		"https://example.com/1.png",
		"https://example.com/2.png",
		"https://example.com/3.png",
		"https://example.com/4.png",
		"https://example.com/5.png",
		"https://example.com/6.png",
		"https://example.com/7.png",
	})

	want := []string{
		"https://example.com/1.png",
		"https://example.com/2.png",
		"https://example.com/3.png",
		"https://example.com/4.png",
		"https://example.com/5.png",
		"https://example.com/6.png",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d photos, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("photo %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestGenerationInputPhotosFromStateFallsBackToLegacyPhotoURL(t *testing.T) {
	got := generationInputPhotosFromState(&State{PhotoURL: "https://example.com/legacy.png"})

	if len(got) != 1 || got[0] != "https://example.com/legacy.png" {
		t.Fatalf("unexpected legacy fallback photos: %#v", got)
	}
}

func TestHandleAwaitingPhotoEditStoresPhotoBatchInState(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
	}
	fc := &Context{
		VkID:  701,
		User:  &User{PaidGens: 1},
		State: &State{},
		Message: &InMessage{Photos: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
			"https://example.com/3.png",
			"https://example.com/4.png",
			"https://example.com/5.png",
			"https://example.com/6.png",
			"https://example.com/7.png",
		}},
	}

	HandleAwaitingPhotoEdit(context.Background(), fc, deps)

	state := stateMgr.states[701]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepAwaitingEditPrompt {
		t.Fatalf("expected %q step, got %q", StepAwaitingEditPrompt, state.Step)
	}
	if state.PhotoURL != "" {
		t.Fatalf("expected result photo url to stay empty, got %q", state.PhotoURL)
	}
	if len(state.InputPhotoURLs) != maxGenerationInputPhotos {
		t.Fatalf("expected %d input photos, got %d", maxGenerationInputPhotos, len(state.InputPhotoURLs))
	}
	if state.InputPhotoURLs[0] != "https://example.com/1.png" || state.InputPhotoURLs[5] != "https://example.com/6.png" {
		t.Fatalf("unexpected stored input photos: %#v", state.InputPhotoURLs)
	}
	if len(sender.screens) == 0 || sender.screens[len(sender.screens)-1].Key != "edit_result_prompt" {
		t.Fatalf("expected edit_result_prompt screen, got %#v", sender.screens)
	}
}

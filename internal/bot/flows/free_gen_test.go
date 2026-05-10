package flows

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

func TestBuildGeneratePayloadKeepsPhotoBatch(t *testing.T) {
	got := buildGeneratePayload(
		11,
		22,
		"google/nano-banana-pro",
		[]string{
			" https://example.com/1.png ",
			"https://example.com/2.png",
			"https://example.com/3.png",
			"https://example.com/4.png",
			"https://example.com/5.png",
			"https://example.com/6.png",
			"https://example.com/7.png",
		},
		"portrait",
		"2k",
		"1:1",
	)

	if got.GenerationID != 11 || got.UserVKID != 22 {
		t.Fatalf("unexpected ids in payload: %#v", got)
	}
	if got.OutputFormat != "png" {
		t.Fatalf("expected png output format, got %q", got.OutputFormat)
	}
	if len(got.Images) != maxGenerationInputPhotos {
		t.Fatalf("expected %d images, got %d: %#v", maxGenerationInputPhotos, len(got.Images), got.Images)
	}
	if got.Images[0] != "https://example.com/1.png" || got.Images[5] != "https://example.com/6.png" {
		t.Fatalf("unexpected images in payload: %#v", got.Images)
	}
}

func TestAppendPendingPhotoBatchCollectsSeparateMessages(t *testing.T) {
	oldDelay := photoBatchCollectDelay
	photoBatchCollectDelay = 0
	defer func() { photoBatchCollectDelay = oldDelay }()

	stateMgr := newFakeStateMgr()
	deps := &Deps{State: stateMgr}
	fc := &Context{
		VkID:  703,
		State: &State{Step: StepAwaitingPhoto, PromptType: "ready_prompt"},
	}

	firstBatchID, ownsBatch := appendPendingPhotoBatch(context.Background(), fc, deps, []string{"https://example.com/1.png"})
	if !ownsBatch {
		t.Fatal("expected first message to wait on the photo batch")
	}
	if firstBatchID == "" {
		t.Fatal("expected photo batch id")
	}

	secondBatchID, ownsBatch := appendPendingPhotoBatch(context.Background(), fc, deps, []string{"https://example.com/2.png"})
	if !ownsBatch {
		t.Fatal("expected second message to wait on the photo batch")
	}
	if secondBatchID == "" || secondBatchID == firstBatchID {
		t.Fatalf("expected refreshed batch id, got first=%q second=%q", firstBatchID, secondBatchID)
	}

	if _, _, ok := waitForPendingPhotoBatch(context.Background(), deps, fc.VkID, firstBatchID); ok {
		t.Fatal("expected stale first batch id to skip generation")
	}

	state, photos, ok := waitForPendingPhotoBatch(context.Background(), deps, fc.VkID, secondBatchID)
	if !ok {
		t.Fatal("expected collected photo batch")
	}
	if state.PhotoBatchID != secondBatchID {
		t.Fatalf("unexpected state batch id %q", state.PhotoBatchID)
	}
	want := []string{"https://example.com/1.png", "https://example.com/2.png"}
	if len(photos) != len(want) {
		t.Fatalf("expected %d photos, got %d: %#v", len(want), len(photos), photos)
	}
	for i := range want {
		if photos[i] != want[i] {
			t.Fatalf("photo %d: expected %q, got %q", i, want[i], photos[i])
		}
	}
}

func TestAppendPendingPhotoBatchCollectsConcurrentMessages(t *testing.T) {
	oldDelay := photoBatchCollectDelay
	photoBatchCollectDelay = 0
	defer func() { photoBatchCollectDelay = oldDelay }()

	stateMgr := newFakeStateMgr()
	deps := &Deps{State: stateMgr}
	fc := &Context{
		VkID:  704,
		State: &State{Step: StepAwaitingPhoto, PromptType: "ready_prompt"},
	}

	const photoCount = 6
	var wg sync.WaitGroup
	wg.Add(photoCount)
	for i := 1; i <= photoCount; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, _ = appendPendingPhotoBatch(context.Background(), fc, deps, []string{fmt.Sprintf("https://example.com/%d.png", i)})
		}()
	}
	wg.Wait()

	state := stateMgr.states[704]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if len(state.InputPhotoURLs) != photoCount {
		t.Fatalf("expected %d photos, got %d: %#v", photoCount, len(state.InputPhotoURLs), state.InputPhotoURLs)
	}
}

type fakePhotoStorage struct {
	uploads []fakePhotoUpload
	failAt  map[int]error
}

type fakePhotoUpload struct {
	key string
	url string
}

func (f *fakePhotoStorage) UploadFromURL(_ context.Context, key, url string) (string, error) {
	f.uploads = append(f.uploads, fakePhotoUpload{key: key, url: url})
	if err := f.failAt[len(f.uploads)-1]; err != nil {
		return "", err
	}
	return key, nil
}

func (f *fakePhotoStorage) PublicURL(key string) string {
	return "https://cdn.example/" + key
}

func TestUploadGenerationInputPhotosUploadsWholeBatch(t *testing.T) {
	storage := &fakePhotoStorage{}
	deps := &Deps{Storage: storage}

	got := uploadGenerationInputPhotos(context.Background(), deps, 705, []string{
		"https://example.com/1.png",
		"https://example.com/2.png",
		"https://example.com/3.png",
	}, "user_upload")

	if len(storage.uploads) != 3 {
		t.Fatalf("expected 3 storage uploads, got %d: %#v", len(storage.uploads), storage.uploads)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 uploaded urls, got %d: %#v", len(got), got)
	}
	for i, upload := range storage.uploads {
		if upload.url != fmt.Sprintf("https://example.com/%d.png", i+1) {
			t.Fatalf("upload %d used unexpected source url %q", i, upload.url)
		}
		wantSuffix := fmt.Sprintf("_%d.png", i+1)
		if len(upload.key) < len(wantSuffix) || upload.key[len(upload.key)-len(wantSuffix):] != wantSuffix {
			t.Fatalf("upload %d key should end with %q, got %q", i, wantSuffix, upload.key)
		}
		if got[i] != "https://cdn.example/"+upload.key {
			t.Fatalf("uploaded url %d: expected public storage url for %q, got %q", i, upload.key, got[i])
		}
	}
}

func TestUploadGenerationInputPhotosKeepsArrayWhenOneUploadFails(t *testing.T) {
	storage := &fakePhotoStorage{failAt: map[int]error{1: errors.New("s3 unavailable")}}
	deps := &Deps{Storage: storage}

	got := uploadGenerationInputPhotos(context.Background(), deps, 706, []string{
		"https://example.com/1.png",
		"https://example.com/2.png",
		"https://example.com/3.png",
	}, "user_upload")

	if len(storage.uploads) != 3 {
		t.Fatalf("expected 3 storage upload attempts, got %d", len(storage.uploads))
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 urls after fallback, got %d: %#v", len(got), got)
	}
	if got[1] != "https://example.com/2.png" {
		t.Fatalf("expected failed upload to fall back to original second url, got %#v", got)
	}
}

func TestGenerationInputPhotosFromStateFallsBackToLegacyPhotoURL(t *testing.T) {
	got := generationInputPhotosFromState(&State{PhotoURL: "https://example.com/legacy.png"})

	if len(got) != 1 || got[0] != "https://example.com/legacy.png" {
		t.Fatalf("unexpected legacy fallback photos: %#v", got)
	}
}

func TestHandleAwaitingPhotoEditStoresPhotoBatchInState(t *testing.T) {
	oldDelay := photoBatchCollectDelay
	photoBatchCollectDelay = 0
	defer func() { photoBatchCollectDelay = oldDelay }()

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
	if state.PhotoBatchID != "" {
		t.Fatalf("expected photo batch id to be cleared, got %q", state.PhotoBatchID)
	}
	if state.InputPhotoURLs[0] != "https://example.com/1.png" || state.InputPhotoURLs[5] != "https://example.com/6.png" {
		t.Fatalf("unexpected stored input photos: %#v", state.InputPhotoURLs)
	}
	if len(sender.screens) == 0 || sender.screens[len(sender.screens)-1].Key != "edit_result_prompt" {
		t.Fatalf("expected edit_result_prompt screen, got %#v", sender.screens)
	}
}

func TestHandleAwaitingPromptWithPhotosProcessesSameMessage(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
	}
	fc := &Context{
		VkID:  702,
		User:  &User{},
		State: &State{Step: StepAwaitingPrompt, PromptType: "custom"},
		Message: &InMessage{
			Text:   "cinematic fashion portrait",
			Photos: []string{"https://example.com/1.png", "https://example.com/2.png"},
		},
	}

	HandleAwaitingPrompt(context.Background(), fc, deps)

	state := stateMgr.states[702]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepAwaitingPhoto {
		t.Fatalf("expected %q step, got %q", StepAwaitingPhoto, state.Step)
	}
	if state.CustomPrompt != "cinematic fashion portrait" {
		t.Fatalf("unexpected custom prompt %q", state.CustomPrompt)
	}
	if len(sender.screens) == 0 || sender.screens[len(sender.screens)-1].Key != "no_gens_left" {
		t.Fatalf("expected no_gens_left screen when prompt+photos are processed immediately, got %#v", sender.screens)
	}
}

func TestResolvedEditPromptPrefersMessageText(t *testing.T) {
	got := resolvedEditPrompt(" add dramatic lighting ", "old prompt")
	if got != "add dramatic lighting" {
		t.Fatalf("expected trimmed message prompt, got %q", got)
	}
}

func TestResolvedEditPromptFallsBackToState(t *testing.T) {
	got := resolvedEditPrompt("   ", " adjust background ")
	if got != "adjust background" {
		t.Fatalf("expected state prompt fallback, got %q", got)
	}
}

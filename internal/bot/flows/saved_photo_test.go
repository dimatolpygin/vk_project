package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

type fakeUserStoreForSavedPhoto struct {
	savedPhotoVKID int64
	savedPhotoURLs []string
	setCalls       int
}

func (f *fakeUserStoreForSavedPhoto) GetByVKID(context.Context, int64) (*repository.User, error) {
	return nil, nil
}

func (f *fakeUserStoreForSavedPhoto) SetGender(context.Context, int64, string) error {
	return nil
}

func (f *fakeUserStoreForSavedPhoto) SetSubscribed(context.Context, int64, bool) error {
	return nil
}

func (f *fakeUserStoreForSavedPhoto) SetSavedPhotos(_ context.Context, vkID int64, urls []string, _ []string) error {
	f.savedPhotoVKID = vkID
	f.savedPhotoURLs = urls
	f.setCalls++
	return nil
}

func (f *fakeUserStoreForSavedPhoto) SetUseSavedPhoto(context.Context, int64, bool) error {
	return nil
}

func (f *fakeUserStoreForSavedPhoto) SaveSettings(context.Context, int64, string, string, string) error {
	return nil
}

func TestHandleSavedPhotoReceivedAcceptsMultiplePhotos(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	userRepo := &fakeUserStoreForSavedPhoto{}
	deps := &Deps{
		Sender:   sender,
		State:    stateMgr,
		UserRepo: userRepo,
	}
	fc := &Context{
		VkID:  801,
		User:  &User{},
		State: &State{Step: StepAwaitingSavedPhoto},
		Message: &InMessage{
			Photos:           []string{"https://example.com/1.png", "https://example.com/2.png"},
			PhotoAttachments: []string{"photo1_1", "photo2_2"},
		},
	}

	HandleSavedPhotoReceived(context.Background(), fc, deps)

	if userRepo.setCalls != 1 {
		t.Fatalf("expected SetSavedPhotos called once, got %d", userRepo.setCalls)
	}
	if len(userRepo.savedPhotoURLs) != 2 {
		t.Fatalf("expected 2 saved URLs, got %d", len(userRepo.savedPhotoURLs))
	}
	if len(sender.screens) == 0 || sender.screens[len(sender.screens)-1].Key != "saved_photo_saved" {
		t.Fatalf("expected saved_photo_saved screen, got %#v", sender.screens)
	}
}

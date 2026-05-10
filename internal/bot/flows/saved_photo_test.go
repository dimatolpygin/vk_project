package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

type fakeUserStoreForSavedPhoto struct {
	savedPhotoVKID int64
	savedPhotoURL  string
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

func (f *fakeUserStoreForSavedPhoto) SetSavedPhoto(_ context.Context, vkID int64, url string) error {
	f.savedPhotoVKID = vkID
	f.savedPhotoURL = url
	f.setCalls++
	return nil
}

func (f *fakeUserStoreForSavedPhoto) SetUseSavedPhoto(context.Context, int64, bool) error {
	return nil
}

func (f *fakeUserStoreForSavedPhoto) SaveSettings(context.Context, int64, string, string, string) error {
	return nil
}

func TestHandleSavedPhotoReceivedRejectsPhotoBatch(t *testing.T) {
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
		Message: &InMessage{Photos: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
		}},
	}

	HandleSavedPhotoReceived(context.Background(), fc, deps)

	if userRepo.setCalls != 0 {
		t.Fatalf("expected saved photo not to be stored, got %d calls", userRepo.setCalls)
	}
	if len(sender.screens) == 0 || sender.screens[len(sender.screens)-1].Key != "saved_photo_batch_not_supported" {
		t.Fatalf("expected saved_photo_batch_not_supported screen, got %#v", sender.screens)
	}
}

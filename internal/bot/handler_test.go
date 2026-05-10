package bot

import (
	"context"
	"encoding/json"
	"testing"

	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/repository"
)

type fakeHandlerUserStore struct {
	getUser          *repository.User
	getErr           error
	createUser       *repository.User
	createErr        error
	createCalls      int
	referralCodeSeen string
}

func (f *fakeHandlerUserStore) GetByVKID(context.Context, int64) (*repository.User, error) {
	return f.getUser, f.getErr
}

func (f *fakeHandlerUserStore) CreateWithReferralCode(_ context.Context, vkID int64, username, firstName, referralCode string) (*repository.User, error) {
	f.createCalls++
	f.referralCodeSeen = referralCode
	if f.createUser != nil || f.createErr != nil {
		return f.createUser, f.createErr
	}
	return &repository.User{
		VKID:         vkID,
		Username:     username,
		FirstName:    firstName,
		ReferralCode: "ref_created",
		Status:       "free",
		FreeGens:     1,
	}, nil
}

type fakeHandlerStateMgr struct {
	states map[int64]*flows.State
}

func newFakeHandlerStateMgr() *fakeHandlerStateMgr {
	return &fakeHandlerStateMgr{states: map[int64]*flows.State{}}
}

func (f *fakeHandlerStateMgr) Get(_ context.Context, vkID int64) (*flows.State, error) {
	if st, ok := f.states[vkID]; ok {
		cp := *st
		return &cp, nil
	}
	return &flows.State{}, nil
}

func (f *fakeHandlerStateMgr) Set(_ context.Context, vkID int64, st *flows.State) error {
	cp := *st
	f.states[vkID] = &cp
	return nil
}

func (f *fakeHandlerStateMgr) SetStep(ctx context.Context, vkID int64, step string) error {
	st, _ := f.Get(ctx, vkID)
	st.Step = step
	return f.Set(ctx, vkID, st)
}

func (f *fakeHandlerStateMgr) Reset(_ context.Context, vkID int64) error {
	delete(f.states, vkID)
	return nil
}

type fakeHandlerFlowSender struct {
	screens []*flows.ScreenMessage
}

func (f *fakeHandlerFlowSender) SendMsg(context.Context, int64, string, string) error  { return nil }
func (f *fakeHandlerFlowSender) SendText(context.Context, int64, string, string) error { return nil }
func (f *fakeHandlerFlowSender) SendPhoto(context.Context, int64, string, string, string) error {
	return nil
}
func (f *fakeHandlerFlowSender) SendScreen(_ context.Context, _ int64, screen *flows.ScreenMessage) error {
	if screen == nil {
		return nil
	}
	cloned := *screen
	f.screens = append(f.screens, &cloned)
	return nil
}
func (f *fakeHandlerFlowSender) SendScreenText(context.Context, int64, string, map[string]any) error {
	return nil
}
func (f *fakeHandlerFlowSender) SendPhotoResult(context.Context, int64, string, string, string, string) error {
	return nil
}

func TestHandleMessageUsesMessageRefForNewUser(t *testing.T) {
	userStore := &fakeHandlerUserStore{}
	stateMgr := newFakeHandlerStateMgr()
	sender := &fakeHandlerFlowSender{}
	registry := flows.NewRegistry(&flows.Deps{
		Sender: sender,
		State:  stateMgr,
	})
	handler := &Handler{
		state:    stateMgr,
		userRepo: userStore,
		registry: registry,
	}

	raw := json.RawMessage(`{"message":{"from_id":101,"peer_id":101,"text":"Привет","ref":"ref_qm7e6euq"}}`)
	handler.handleMessage(context.Background(), raw)

	if userStore.createCalls != 1 {
		t.Fatalf("expected one create call, got %d", userStore.createCalls)
	}
	if userStore.referralCodeSeen != "ref_qm7e6euq" {
		t.Fatalf("expected referral code from message.ref, got %q", userStore.referralCodeSeen)
	}
	if len(sender.screens) == 0 || sender.screens[0].Key != "welcome" {
		t.Fatalf("expected welcome screen to be sent, got %#v", sender.screens)
	}
	if st := stateMgr.states[101]; st == nil || st.Step != flows.StepWelcome {
		t.Fatalf("expected state step %q, got %#v", flows.StepWelcome, st)
	}
}

func TestHandleMessageDoesNotReattachExistingUser(t *testing.T) {
	userStore := &fakeHandlerUserStore{
		getUser: &repository.User{
			VKID:         102,
			ReferralCode: "ref_existing",
			Status:       "free",
			FreeGens:     1,
		},
	}
	stateMgr := newFakeHandlerStateMgr()
	sender := &fakeHandlerFlowSender{}
	registry := flows.NewRegistry(&flows.Deps{
		Sender: sender,
		State:  stateMgr,
	})
	handler := &Handler{
		state:    stateMgr,
		userRepo: userStore,
		registry: registry,
	}

	raw := json.RawMessage(`{"message":{"from_id":102,"peer_id":102,"text":"Привет","ref":"ref_other"}}`)
	handler.handleMessage(context.Background(), raw)

	if userStore.createCalls != 0 {
		t.Fatalf("expected existing user to skip create, got %d create calls", userStore.createCalls)
	}
}

func TestExtractPhotosKeepsAllPhotoAttachments(t *testing.T) {
	got := extractPhotos([]VKAttachment{
		{
			Type: "photo",
			Photo: VKPhoto{Sizes: []VKPhotoSize{
				{URL: "https://example.com/1-small.jpg", Width: 75, Height: 75},
				{URL: "https://example.com/1-large.jpg", Width: 1280, Height: 960},
			}},
		},
		{
			Type: "doc",
		},
		{
			Type: "photo",
			Photo: VKPhoto{Sizes: []VKPhotoSize{
				{URL: "https://example.com/2-medium.jpg", Width: 604, Height: 453},
				{URL: "https://example.com/2-large.jpg", Width: 807, Height: 605},
			}},
		},
	})

	want := []string{"https://example.com/1-large.jpg", "https://example.com/2-large.jpg"}
	if len(got) != len(want) {
		t.Fatalf("expected %d photos, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("photo %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestExtractPhotosSupportsLegacyVKPhotoURLs(t *testing.T) {
	got := extractPhotos([]VKAttachment{
		{Type: "photo", Photo: VKPhoto{Photo604: "https://example.com/1-604.jpg", Photo1280: "https://example.com/1-1280.jpg"}},
		{Type: "photo", Photo: VKPhoto{SrcSmall: "https://example.com/2-small.jpg", SrcBig: "https://example.com/2-big.jpg"}},
	})

	want := []string{"https://example.com/1-1280.jpg", "https://example.com/2-big.jpg"}
	if len(got) != len(want) {
		t.Fatalf("expected %d photos, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("photo %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

package flows

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"vk_neuro_bot/internal/repository"
)

type fakeSender struct {
	screens []*ScreenMessage
}

func (f *fakeSender) SendMsg(context.Context, int64, string, string) error           { return nil }
func (f *fakeSender) SendText(context.Context, int64, string, string) error          { return nil }
func (f *fakeSender) SendPhoto(context.Context, int64, string, string, string) error { return nil }
func (f *fakeSender) SendScreen(_ context.Context, _ int64, screen *ScreenMessage) error {
	if screen == nil {
		return nil
	}
	cloned := *screen
	f.screens = append(f.screens, &cloned)
	return nil
}
func (f *fakeSender) SendScreenText(context.Context, int64, string, map[string]any) error { return nil }
func (f *fakeSender) SendPhotoResult(context.Context, int64, string, string, string, string) error {
	return nil
}

type fakeStateMgr struct {
	states map[int64]*State
}

func newFakeStateMgr() *fakeStateMgr {
	return &fakeStateMgr{states: map[int64]*State{}}
}

func (f *fakeStateMgr) Get(_ context.Context, vkID int64) (*State, error) {
	if st, ok := f.states[vkID]; ok {
		cp := *st
		return &cp, nil
	}
	return &State{}, nil
}

func (f *fakeStateMgr) Set(_ context.Context, vkID int64, st *State) error {
	cp := *st
	f.states[vkID] = &cp
	return nil
}

func (f *fakeStateMgr) SetStep(ctx context.Context, vkID int64, step string) error {
	st, _ := f.Get(ctx, vkID)
	st.Step = step
	return f.Set(ctx, vkID, st)
}

func (f *fakeStateMgr) Reset(_ context.Context, vkID int64) error {
	delete(f.states, vkID)
	return nil
}

type fakeCategoryRepo struct {
	activeByGender map[string][]*repository.Category
	couple         []*repository.Category
}

func (f *fakeCategoryRepo) ListActive(_ context.Context, gender string) ([]*repository.Category, error) {
	return f.activeByGender[gender], nil
}

func (f *fakeCategoryRepo) ListActiveCouple(context.Context) ([]*repository.Category, error) {
	return f.couple, nil
}

type fakePromptRepo struct {
	byCategoryGender map[string][]*repository.Prompt
	byID             map[int]*repository.Prompt
}

func (f *fakePromptRepo) ListByCategory(_ context.Context, categoryID int, gender string) ([]*repository.Prompt, error) {
	return f.byCategoryGender[promptKey(categoryID, gender)], nil
}

func (f *fakePromptRepo) GetByID(_ context.Context, id int) (*repository.Prompt, error) {
	return f.byID[id], nil
}

func promptKey(categoryID int, gender string) string {
	return strconv.Itoa(categoryID) + ":" + gender
}

func TestHandleReadyPromptsMenuShowsFirstCategoryPage(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
		CatRepo: &fakeCategoryRepo{
			activeByGender: map[string][]*repository.Category{
				"male": makeCategories(6),
			},
		},
	}
	fc := &Context{
		VkID:  101,
		User:  &User{Gender: "male", FreeGens: 1},
		State: &State{},
	}

	HandleReadyPromptsMenu(context.Background(), fc, deps)

	state := stateMgr.states[101]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepReadyPromptsCategories {
		t.Fatalf("expected %q step, got %q", StepReadyPromptsCategories, state.Step)
	}
	if state.CategoryPage != 1 || state.PromptPage != 1 {
		t.Fatalf("expected category/prompt page to be 1, got %d/%d", state.CategoryPage, state.PromptPage)
	}

	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if len(keyboard.Buttons) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(keyboard.Buttons))
	}
	if got := keyboard.Buttons[0][0].Action.Label; got != "Category 1" {
		t.Fatalf("expected first category on page 1, got %q", got)
	}
	if got := keyboard.Buttons[2][0].Action.Label; got != "Вперёд ➡️" {
		t.Fatalf("expected forward pager row, got %q", got)
	}
	if got := keyboard.Buttons[3][0].Action.Payload; got != cbPayload("back") {
		t.Fatalf("expected service back row last, got %q", got)
	}
}

func TestHandleSelectCategoryShowsFirstPromptPageAndStoresState(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
		PromptRepo: &fakePromptRepo{
			byCategoryGender: map[string][]*repository.Prompt{
				promptKey(7, "male"): makePrompts(7, 6),
			},
			byID: map[int]*repository.Prompt{},
		},
	}
	fc := &Context{
		VkID:  102,
		User:  &User{Gender: "male", FreeGens: 1},
		State: &State{CategoryPage: 2},
		Callback: &CallbackData{
			CategoryID: 7,
		},
	}

	HandleSelectCategory(context.Background(), fc, deps)

	state := stateMgr.states[102]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepReadyPromptsPrompts {
		t.Fatalf("expected %q step, got %q", StepReadyPromptsPrompts, state.Step)
	}
	if state.CategoryID != 7 {
		t.Fatalf("expected category id 7, got %d", state.CategoryID)
	}
	if state.CategoryPage != 2 || state.PromptPage != 1 {
		t.Fatalf("expected category page 2 and prompt page 1, got %d/%d", state.CategoryPage, state.PromptPage)
	}

	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if len(keyboard.Buttons) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(keyboard.Buttons))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(keyboard.Buttons[2][0].Action.Payload), &payload); err != nil {
		t.Fatalf("unmarshal pager payload: %v", err)
	}
	if got := payload["type"]; got != "prompts_page" {
		t.Fatalf("expected prompts_page payload type, got %#v", got)
	}
	if got := int(payload["category_id"].(float64)); got != 7 {
		t.Fatalf("expected pager payload category 7, got %d", got)
	}
}

func TestHandleBackFromPromptListReturnsToStoredCategoryPage(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
		CatRepo: &fakeCategoryRepo{
			activeByGender: map[string][]*repository.Category{
				"male": makeCategories(6),
			},
		},
	}
	fc := &Context{
		VkID: 103,
		User: &User{Gender: "male", FreeGens: 1},
		State: &State{
			Step:         StepReadyPromptsPrompts,
			PromptType:   "ready_prompt",
			CategoryPage: 2,
			CategoryID:   7,
			PromptPage:   1,
		},
	}

	HandleBack(context.Background(), fc, deps)

	state := stateMgr.states[103]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepReadyPromptsCategories {
		t.Fatalf("expected %q step, got %q", StepReadyPromptsCategories, state.Step)
	}
	if state.CategoryPage != 2 {
		t.Fatalf("expected category page 2, got %d", state.CategoryPage)
	}

	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Category 5" {
		t.Fatalf("expected second page to start from category 5, got %q", got)
	}
	if got := keyboard.Buttons[1][0].Action.Label; got != "⬅️ Назад" {
		t.Fatalf("expected backward pager on second page, got %q", got)
	}
}

func TestHandleCoupleStartUsesPaginatedCategories(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
		CatRepo: &fakeCategoryRepo{
			couple: makeCategories(6),
		},
	}
	fc := &Context{
		VkID:  104,
		User:  &User{FreeGens: 1},
		State: &State{},
	}

	HandleCoupleStart(context.Background(), fc, deps)

	state := stateMgr.states[104]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepCoupleCategories {
		t.Fatalf("expected %q step, got %q", StepCoupleCategories, state.Step)
	}
	if state.PromptType != "couple" {
		t.Fatalf("expected prompt type couple, got %q", state.PromptType)
	}

	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	var payload map[string]any
	if err := json.Unmarshal([]byte(keyboard.Buttons[2][0].Action.Payload), &payload); err != nil {
		t.Fatalf("unmarshal couple pager payload: %v", err)
	}
	if got := payload["type"]; got != "couple_page" {
		t.Fatalf("expected couple_page payload type, got %#v", got)
	}
}

func decodeKeyboard(t *testing.T, raw string) Keyboard {
	t.Helper()
	var keyboard Keyboard
	if err := json.Unmarshal([]byte(raw), &keyboard); err != nil {
		t.Fatalf("unmarshal keyboard: %v", err)
	}
	return keyboard
}

func makeCategories(n int) []*repository.Category {
	out := make([]*repository.Category, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, &repository.Category{ID: i, Name: "Category " + strconv.Itoa(i)})
	}
	return out
}

func makePrompts(categoryID, n int) []*repository.Prompt {
	out := make([]*repository.Prompt, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, &repository.Prompt{ID: i, CategoryID: categoryID, Name: "Prompt " + strconv.Itoa(i)})
	}
	return out
}

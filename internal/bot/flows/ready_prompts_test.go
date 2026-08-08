package flows

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
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

// fakeCategoryRepo изображает дерево разделов: корни лежат по разделам, дети —
// в children под id родителя. Раздел и prompt_gender проставляются так же, как их
// проставила пользователям миграция 00029.
type fakeCategoryRepo struct {
	readyByGender  map[string][]*repository.Category
	activeByGender map[string][]*repository.Category
	couple         []*repository.Category
	kids           []*repository.Category
	greetings      []*repository.Category
	children       map[int][]*repository.Category
}

func (f *fakeCategoryRepo) selfRoots(gender string) []*repository.Category {
	cats := f.readyByGender[gender]
	if f.readyByGender == nil {
		cats = f.activeByGender[gender]
	}
	return stampSection(cats, repository.SectionSelf, nil)
}

func (f *fakeCategoryRepo) coupleRoots() []*repository.Category {
	couple := repository.GenderCouple
	return stampSection(f.couple, repository.SectionCouple, &couple)
}

func stampSection(cats []*repository.Category, section string, promptGender *string) []*repository.Category {
	for _, cat := range cats {
		if cat.Section == "" {
			cat.Section = section
		}
		if cat.PromptGender == nil {
			cat.PromptGender = promptGender
		}
	}
	return cats
}

func (f *fakeCategoryRepo) ListReadyPromptCategories(_ context.Context, gender string) ([]*repository.Category, error) {
	return f.selfRoots(gender), nil
}

func (f *fakeCategoryRepo) ListActive(_ context.Context, gender string) ([]*repository.Category, error) {
	return f.activeByGender[gender], nil
}

func (f *fakeCategoryRepo) ListActiveCouple(context.Context) ([]*repository.Category, error) {
	return f.coupleRoots(), nil
}

func (f *fakeCategoryRepo) ListRoots(_ context.Context, section string, filter repository.CategoryFilter) ([]*repository.Category, error) {
	switch section {
	case repository.SectionCouple:
		return f.coupleRoots(), nil
	case repository.SectionKids:
		return stampSection(f.kids, repository.SectionKids, nil), nil
	case repository.SectionGreetings:
		return stampSection(f.greetings, repository.SectionGreetings, nil), nil
	}
	return f.selfRoots(filter.Gender), nil
}

func (f *fakeCategoryRepo) ListChildren(_ context.Context, parentID int, _ repository.CategoryFilter) ([]*repository.Category, error) {
	return f.children[parentID], nil
}

func (f *fakeCategoryRepo) GetByID(_ context.Context, id int) (*repository.Category, error) {
	for _, cat := range f.allCategories() {
		if cat.ID == id {
			return cat, nil
		}
	}
	return nil, nil
}

func (f *fakeCategoryRepo) Path(ctx context.Context, id int) ([]*repository.Category, error) {
	var path []*repository.Category
	for node, _ := f.GetByID(ctx, id); node != nil; {
		path = append([]*repository.Category{node}, path...)
		if node.ParentID == nil {
			break
		}
		node, _ = f.GetByID(ctx, *node.ParentID)
	}
	return path, nil
}

func (f *fakeCategoryRepo) allCategories() []*repository.Category {
	var all []*repository.Category
	for gender := range f.readyByGender {
		all = append(all, f.selfRoots(gender)...)
	}
	for gender := range f.activeByGender {
		all = append(all, stampSection(f.activeByGender[gender], repository.SectionSelf, nil)...)
	}
	all = append(all, f.coupleRoots()...)
	all = append(all, stampSection(f.kids, repository.SectionKids, nil)...)
	all = append(all, stampSection(f.greetings, repository.SectionGreetings, nil)...)
	for _, children := range f.children {
		all = append(all, children...)
	}
	return all
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
			readyByGender: map[string][]*repository.Category{
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

	// Четыре категории по одной в ряд + ряд листалки + служебная «В меню» —
	// ровно шесть строк, предел inline-клавиатуры ВК.
	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if len(keyboard.Buttons) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(keyboard.Buttons))
	}
	if got := keyboard.Buttons[0][0].Action.Label; got != "Category 1" {
		t.Fatalf("expected the list to start from the first category, got %q", got)
	}
	if got := keyboard.Buttons[4][0].Action.Label; got != "Вперёд ➡️" {
		t.Fatalf("expected the pager row under the list, got %q", got)
	}
	if got := keyboard.Buttons[5][0].Action.Payload; got != cbPayload("back") {
		t.Fatalf("expected service back row last, got %q", got)
	}
}

func TestHandleSelectCategoryShowsFirstPromptPageAndStoresState(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
		CatRepo: &fakeCategoryRepo{
			readyByGender: map[string][]*repository.Category{
				"male": {{ID: 7, Name: "Category 7"}},
			},
		},
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
	if len(keyboard.Buttons) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(keyboard.Buttons))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(keyboard.Buttons[4][0].Action.Payload), &payload); err != nil {
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
			readyByGender: map[string][]*repository.Category{
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

	// Последняя страница: «Вперёд» не показываем, ряд листалки с одним «Назад»
	// стоит под списком, служебная «В меню» — последней.
	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Category 5" {
		t.Fatalf("expected second page to start from category 5, got %q", got)
	}
	if got := keyboard.Buttons[2][0].Action.Label; got != "⬅️ Назад" {
		t.Fatalf("expected the pager row under the list, got %q", got)
	}
	if got := keyboard.Buttons[3][0].Action.Payload; got != cbPayload("back") {
		t.Fatalf("expected service back row last, got %q", got)
	}
}

func TestHandleCoupleStartAwaitsPhotoBeforeCategories(t *testing.T) {
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
	if state.Step != StepCoupleAwaitingPhoto {
		t.Fatalf("expected %q step, got %q", StepCoupleAwaitingPhoto, state.Step)
	}
	if state.PromptType != "couple" {
		t.Fatalf("expected prompt type couple, got %q", state.PromptType)
	}

	if len(sender.screens) == 0 {
		t.Fatal("expected a screen to be sent")
	}
	last := sender.screens[len(sender.screens)-1]
	if last.Key != "couple_intro" {
		t.Fatalf("expected couple_intro screen, got %q", last.Key)
	}
	// На шаге ожидания фото категории показываться не должны.
	keyboard := decodeKeyboard(t, last.Keyboard)
	for _, row := range keyboard.Buttons {
		for _, btn := range row {
			if strings.HasPrefix(btn.Action.Label, "Category ") {
				t.Fatalf("did not expect category buttons before photo upload, got %q", btn.Action.Label)
			}
		}
	}
}

func TestHandleCoupleAwaitingPhotoShowsPaginatedCategories(t *testing.T) {
	oldDelay := photoBatchCollectDelay
	photoBatchCollectDelay = 0
	defer func() { photoBatchCollectDelay = oldDelay }()

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
		VkID:  114,
		User:  &User{FreeGens: 1},
		State: &State{Step: StepCoupleAwaitingPhoto, PromptType: "couple"},
		Message: &InMessage{
			Photos: []string{"https://example.com/couple1.jpg", "https://example.com/couple2.jpg"},
		},
	}

	HandleCoupleAwaitingPhoto(context.Background(), fc, deps)

	state := stateMgr.states[114]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepCoupleCategories {
		t.Fatalf("expected %q step, got %q", StepCoupleCategories, state.Step)
	}
	if len(state.CouplePhotoURLs) != 2 {
		t.Fatalf("expected 2 stored couple photos, got %d", len(state.CouplePhotoURLs))
	}

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "couple_submenu" {
		t.Fatalf("expected couple_submenu screen, got %q", last.Key)
	}
	keyboard := decodeKeyboard(t, last.Keyboard)
	var payload map[string]any
	if err := json.Unmarshal([]byte(keyboard.Buttons[4][0].Action.Payload), &payload); err != nil {
		t.Fatalf("unmarshal couple pager payload: %v", err)
	}
	if got := payload["type"]; got != "couple_page" {
		t.Fatalf("expected couple_page payload type, got %#v", got)
	}
}

func TestHandleReadyPromptsMenuUnknownGenderRequestsSelection(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender:  sender,
		State:   stateMgr,
		CatRepo: &fakeCategoryRepo{},
	}
	fc := &Context{
		VkID:  105,
		User:  &User{Gender: "unknown", FreeGens: 1},
		State: &State{},
	}

	HandleReadyPromptsMenu(context.Background(), fc, deps)

	state := stateMgr.states[105]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepAwaitingGender {
		t.Fatalf("expected %q step, got %q", StepAwaitingGender, state.Step)
	}
	if state.PromptType != "ready_prompt" {
		t.Fatalf("expected prompt type ready_prompt, got %q", state.PromptType)
	}
	if len(sender.screens) == 0 {
		t.Fatal("expected a screen to be sent")
	}
	if got := sender.screens[len(sender.screens)-1].Key; got != "gender_select" {
		t.Fatalf("expected gender_select screen, got %q", got)
	}
}

func TestHandleGenderSelectAfterReadyPromptsShowsCategories(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender: sender,
		State:  stateMgr,
		CatRepo: &fakeCategoryRepo{
			readyByGender: map[string][]*repository.Category{
				"male": makeCategories(6),
			},
		},
	}
	fc := &Context{
		VkID:  106,
		User:  &User{Gender: "unknown", FreeGens: 1},
		State: &State{PromptType: "ready_prompt"},
	}

	HandleGenderSelect(context.Background(), fc, deps, "male")

	state := stateMgr.states[106]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepReadyPromptsCategories {
		t.Fatalf("expected %q step, got %q", StepReadyPromptsCategories, state.Step)
	}
	if state.CategoryPage != 1 || state.PromptPage != 1 {
		t.Fatalf("expected category/prompt page to be 1, got %d/%d", state.CategoryPage, state.PromptPage)
	}
	if len(sender.screens) == 0 {
		t.Fatal("expected a screen to be sent")
	}
	if got := sender.screens[len(sender.screens)-1].Key; got != "ready_prompts_intro" {
		t.Fatalf("expected ready_prompts_intro screen, got %q", got)
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

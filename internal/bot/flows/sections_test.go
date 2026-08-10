package flows

import (
	"context"
	"encoding/json"
	"testing"

	"vk_neuro_bot/internal/repository"
)

func strptr(v string) *string { return &v }

// couplePhotos — фото, загруженные на входе в режим съёмки. С этапа 13 без них
// раздел до дерева не доходит: сначала режим, потом фото, потом шаблоны.
func couplePhotos() []string { return []string{"https://cdn.example/couple-1.jpg"} }
func intptr(v int) *int      { return &v }

// treeRepo — раздел «Парное фото» с подменю: корень 10 с двумя детьми,
// у первого ребёнка есть промты.
func treeRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{
		couple: []*repository.Category{
			{ID: 10, Name: "Парное фото", Section: repository.SectionCouple, PromptGender: strptr(repository.GenderCouple)},
		},
		children: map[int][]*repository.Category{
			10: {
				{ID: 11, Name: "Классика", Section: repository.SectionCouple, ParentID: intptr(10)},
				{ID: 12, Name: "Поколения", Section: repository.SectionCouple, ParentID: intptr(10)},
			},
		},
	}
}

func TestOpenCategoryShowsChildrenWhenNodeHasThem(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: treeRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID: 201,
		User: &User{Gender: "male", FreeGens: 1},
		// Фото загружено на входе в режим — навигация по дереву идёт после него.
		State:    &State{Step: StepCoupleCategories, PromptType: "couple", CouplePhotoURLs: couplePhotos()},
		Callback: &CallbackData{CategoryID: 10},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	state := stateMgr.states[201]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.SectionID != 10 {
		t.Fatalf("expected the opened node to be remembered, got %d", state.SectionID)
	}
	if state.Section != repository.SectionCouple {
		t.Fatalf("expected couple section, got %q", state.Section)
	}
	if state.Step != StepCoupleCategories {
		t.Fatalf("expected %q step, got %q", StepCoupleCategories, state.Step)
	}

	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Классика" {
		t.Fatalf("expected the first child button, got %q", got)
	}
	if got := keyboard.Buttons[1][0].Action.Label; got != "Поколения" {
		t.Fatalf("expected the second child button, got %q", got)
	}
}

func TestOpenCategoryShowsPromptsWhenNodeIsLeaf(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender:  sender,
		State:   stateMgr,
		CatRepo: treeRepo(),
		PromptRepo: &fakePromptRepo{
			byCategoryGender: map[string][]*repository.Prompt{
				// Пол промтов пришёл от корня раздела, а не от пола пользователя.
				promptKey(11, repository.GenderCouple): makePrompts(11, 3),
			},
		},
	}
	fc := &Context{
		VkID:     202,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepCoupleCategories, PromptType: "couple", CategoryPage: 1, CouplePhotoURLs: couplePhotos()},
		Callback: &CallbackData{CategoryID: 11},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	state := stateMgr.states[202]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.Step != StepCouplePrompts {
		t.Fatalf("expected %q step, got %q", StepCouplePrompts, state.Step)
	}
	if state.CategoryID != 11 || state.SectionID != 10 {
		t.Fatalf("expected category 11 under node 10, got %d under %d", state.CategoryID, state.SectionID)
	}
	if got := sender.screens[len(sender.screens)-1].Key; got != "prompts_list" {
		t.Fatalf("expected prompts_list screen, got %q", got)
	}
}

func TestBackFromNestedPromptsReturnsToParentNode(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: treeRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID: 203,
		User: &User{Gender: "male", FreeGens: 1},
		State: &State{
			Step:            StepCouplePrompts,
			PromptType:      "couple",
			Section:         repository.SectionCouple,
			SectionID:       10,
			CategoryID:      11,
			CouplePhotoURLs: couplePhotos(),
		},
	}

	HandleBack(context.Background(), fc, deps)

	state := stateMgr.states[203]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	// Ровно на уровень выше: снова подменю узла 10, а не корень раздела.
	if state.SectionID != 10 || state.Step != StepCoupleCategories {
		t.Fatalf("expected to return to node 10 submenu, got node %d step %q", state.SectionID, state.Step)
	}
	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Классика" {
		t.Fatalf("expected the submenu of node 10, got %q", got)
	}
}

func TestBackFromSubmenuReturnsToSectionRoot(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: treeRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID: 204,
		User: &User{Gender: "male", FreeGens: 1},
		State: &State{
			Step:       StepCoupleCategories,
			PromptType: "couple",
			Section:    repository.SectionCouple,
			SectionID:  10,
		},
	}

	HandleBack(context.Background(), fc, deps)

	state := stateMgr.states[204]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	if state.SectionID != 0 {
		t.Fatalf("expected to land on the section root, got node %d", state.SectionID)
	}
	if got := sender.screens[len(sender.screens)-1].Key; got != "couple_submenu" {
		t.Fatalf("expected couple_submenu screen, got %q", got)
	}
}

func TestSubmenuPagerCarriesParentID(t *testing.T) {
	repo := treeRepo()
	repo.children[10] = append(repo.children[10],
		&repository.Category{ID: 13, Name: "Семейное", Section: repository.SectionCouple, ParentID: intptr(10)},
		&repository.Category{ID: 14, Name: "Свадьба", Section: repository.SectionCouple, ParentID: intptr(10)},
		&repository.Category{ID: 15, Name: "Юбилей", Section: repository.SectionCouple, ParentID: intptr(10)},
	)

	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: repo, PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     205,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepCoupleCategories, PromptType: "couple", CouplePhotoURLs: couplePhotos()},
		Callback: &CallbackData{CategoryID: 10},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	var payload map[string]any
	if err := json.Unmarshal([]byte(keyboard.Buttons[4][0].Action.Payload), &payload); err != nil {
		t.Fatalf("unmarshal submenu pager payload: %v", err)
	}
	if got := payload["type"]; got != "open_category_page" {
		t.Fatalf("expected open_category_page pager, got %#v", got)
	}
	// Без родителя в payload вторая страница подменю не найдёт свой уровень.
	if got := int(payload["category_id"].(float64)); got != 10 {
		t.Fatalf("expected parent id 10 in the pager payload, got %d", got)
	}
}

func TestNodeScreenKeyFallsBackToSectionScreen(t *testing.T) {
	spec := specForSection(repository.SectionCouple)

	if got := nodeScreenKey(&repository.Category{ID: 1}, spec); got != "couple_submenu" {
		t.Fatalf("expected the section screen as a fallback, got %q", got)
	}
	own := &repository.Category{ID: 2, ScreenKey: strptr("kids_intro")}
	if got := nodeScreenKey(own, spec); got != "kids_intro" {
		t.Fatalf("expected the node's own screen, got %q", got)
	}
}

func TestResolveNodeGenderInheritsFromAncestor(t *testing.T) {
	repo := treeRepo()
	deps := &Deps{CatRepo: repo}
	fc := &Context{User: &User{Gender: "male"}}
	ctx := context.Background()

	child, _ := repo.GetByID(ctx, 11)
	if got := resolveNodeGender(ctx, deps, child, fc); got != repository.GenderCouple {
		t.Fatalf("expected the child to inherit couple gender, got %q", got)
	}

	// Узел без переопределения по-прежнему фильтрует промты полом пользователя.
	standalone := &repository.Category{ID: 20, Section: repository.SectionSelf}
	if got := resolveNodeGender(ctx, deps, standalone, fc); got != "male" {
		t.Fatalf("expected the user gender, got %q", got)
	}
}

// emptyNodeRepo — раздел «Парное фото» из этапа 6: наполненный узел и два пустых,
// какими «Семейное фото» и «Фото поколений» приезжают из миграции 00030.
func emptyNodeRepo() *fakeCategoryRepo {
	repo := treeRepo()
	repo.couple = append(repo.couple,
		&repository.Category{ID: 20, Name: "Семейное фото", Section: repository.SectionCouple, PromptGender: strptr(repository.GenderCouple)},
		&repository.Category{ID: 21, Name: "Фото поколений", Section: repository.SectionCouple, PromptGender: strptr(repository.GenderCouple)},
	)
	return repo
}

func TestEmptyNodeShowsSoonScreenInsteadOfEmptyKeyboard(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: emptyNodeRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     206,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepCoupleCategories, PromptType: "couple", Section: repository.SectionCouple, CouplePhotoURLs: couplePhotos()},
		Callback: &CallbackData{CategoryID: 20},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "section_soon" {
		t.Fatalf("expected the section_soon screen, got %q", got)
	}
	state := stateMgr.states[206]
	if state == nil {
		t.Fatal("expected state to be saved")
	}
	// Без записанного состояния «Назад» на этом экране уводит в главное меню
	// мимо уровня, с которого пользователь пришёл.
	if state.CategoryID != 20 {
		t.Fatalf("expected the empty node to be remembered, got %d", state.CategoryID)
	}
	if state.Step != StepCouplePrompts {
		t.Fatalf("expected %q step, got %q", StepCouplePrompts, state.Step)
	}
}

func TestBackFromEmptyNodeReturnsToSubmenu(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: emptyNodeRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     207,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepCoupleCategories, PromptType: "couple", Section: repository.SectionCouple, CouplePhotoURLs: couplePhotos()},
		Callback: &CallbackData{CategoryID: 20},
	}

	HandleOpenCategory(context.Background(), fc, deps)
	fc.Callback = &CallbackData{Type: "back"}
	HandleBack(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "couple_submenu" {
		t.Fatalf("expected to return to the couple submenu, got %q", got)
	}
	keyboard := decodeKeyboard(t, sender.screens[len(sender.screens)-1].Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Парное фото" {
		t.Fatalf("expected the submenu buttons, got %q", got)
	}
}

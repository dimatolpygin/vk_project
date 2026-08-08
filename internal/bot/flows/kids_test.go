package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

// kidsRepo — раздел «Детские фото» после миграции 00031: два корня, пол задан
// на узле, промты есть только у «Мальчика».
func kidsRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{
		kids: []*repository.Category{
			{ID: 60, Name: "Мальчик", Section: repository.SectionKids, PromptGender: strptr("male")},
			{ID: 61, Name: "Девочка", Section: repository.SectionKids, PromptGender: strptr("female")},
		},
	}
}

func TestKidsMenuShowsNodesWithoutAskingUserGender(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: kidsRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID: 301,
		// Пол ребёнка задают кнопки, поэтому неизвестный пол пользователя не должен
		// уводить на экран выбора пола, как в разделе «Фото для себя».
		User:     &User{Gender: "unknown", FreeGens: 1},
		State:    &State{Step: StepMainMenu},
		Callback: &CallbackData{Type: "kids"},
	}

	HandleKidsMenu(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "kids_intro" {
		t.Fatalf("expected the kids_intro screen, got %q", last.Key)
	}
	keyboard := decodeKeyboard(t, last.Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Мальчик" {
		t.Fatalf("expected the first kids node, got %q", got)
	}
	if got := keyboard.Buttons[1][0].Action.Label; got != "Девочка" {
		t.Fatalf("expected the second kids node, got %q", got)
	}
	state := stateMgr.states[301]
	if state == nil || state.Section != repository.SectionKids {
		t.Fatalf("expected the kids section in state, got %#v", state)
	}
	if state.Step != StepKidsCategories {
		t.Fatalf("expected %q step, got %q", StepKidsCategories, state.Step)
	}
}

func TestKidsPromptsUseNodeGenderNotUserGender(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender:  sender,
		State:   stateMgr,
		CatRepo: kidsRepo(),
		PromptRepo: &fakePromptRepo{
			byCategoryGender: map[string][]*repository.Prompt{
				promptKey(60, "male"): makePrompts(60, 2),
			},
		},
	}
	fc := &Context{
		VkID:     302,
		User:     &User{Gender: "female", FreeGens: 1},
		State:    &State{Step: StepKidsCategories, Section: repository.SectionKids, PromptType: "kids"},
		Callback: &CallbackData{CategoryID: 60},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	// Промты нашлись только по ключу «male» — значит, пол пришёл с узла «Мальчик»,
	// а не от пользователя-женщины.
	if got := sender.screens[len(sender.screens)-1].Key; got != "prompts_list" {
		t.Fatalf("expected the prompt list, got %q", got)
	}
	state := stateMgr.states[302]
	if state == nil || state.Step != StepKidsPrompts {
		t.Fatalf("expected %q step, got %#v", StepKidsPrompts, state)
	}
}

func TestEmptyKidsNodeShowsSoonScreen(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: kidsRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     303,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepKidsCategories, Section: repository.SectionKids, PromptType: "kids"},
		Callback: &CallbackData{CategoryID: 61},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "section_soon" {
		t.Fatalf("expected the section_soon screen, got %q", got)
	}
}

func TestKidsSectionIgnoresSavedPhoto(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender:     sender,
		State:      stateMgr,
		CatRepo:    kidsRepo(),
		PromptRepo: &fakePromptRepo{byID: map[int]*repository.Prompt{7: {ID: 7, Name: "Космонавт", Prompt: "kid in space suit"}}},
	}
	fc := &Context{
		VkID: 304,
		// Сохранённое фото — фото взрослого, для детского раздела оно не подходит.
		User:     &User{Gender: "male", FreeGens: 1, UseSavedPhoto: true, SavedPhotoURLs: []string{"https://example.com/adult.jpg"}},
		State:    &State{Step: StepKidsPrompts, Section: repository.SectionKids, PromptType: "kids", CategoryID: 60},
		Callback: &CallbackData{PromptID: 7},
	}

	HandleSelectPrompt(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "photo_requirements" {
		t.Fatalf("expected the bot to ask for a new photo, got %q", got)
	}
}

func TestBackFromKidsRootLeavesToMainMenu(t *testing.T) {
	// Раздел новый, а «Назад» с его корня должен вести наружу — это и проверяет,
	// что выход из дерева перестал быть списком известных шагов.
	if !IsSectionStep(StepKidsCategories) || !IsSectionStep(StepKidsPrompts) {
		t.Fatal("expected the kids steps to be recognised as section steps")
	}
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: kidsRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     305,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepKidsCategories, Section: repository.SectionKids},
		Callback: &CallbackData{Type: "back"},
	}

	if sectionBack(context.Background(), fc, deps) {
		t.Fatal("expected the root level to fall through to the main menu")
	}
}

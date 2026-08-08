package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

// greetingsRepo — раздел после миграции 00032: адресаты в корне, праздники под
// ними. У праздников пола нет — он наследуется от адресата.
func greetingsRepo() *fakeCategoryRepo {
	male := &repository.Category{ID: 70, Name: "Мужчине", Section: repository.SectionGreetings,
		PromptGender: strptr("male"), ScreenKey: strptr("greetings_holidays")}
	female := &repository.Category{ID: 71, Name: "Женщине", Section: repository.SectionGreetings,
		PromptGender: strptr("female"), ScreenKey: strptr("greetings_holidays")}
	return &fakeCategoryRepo{
		greetings: []*repository.Category{male, female},
		children: map[int][]*repository.Category{
			70: {
				{ID: 72, Name: "День рождения", Section: repository.SectionGreetings, ParentID: intptr(70)},
				{ID: 73, Name: "23 февраля", Section: repository.SectionGreetings, ParentID: intptr(70)},
			},
			71: {
				{ID: 74, Name: "День рождения", Section: repository.SectionGreetings, ParentID: intptr(71)},
			},
		},
	}
}

func greetingsDeps(sender *fakeSender, stateMgr *fakeStateMgr, prompts *fakePromptRepo) *Deps {
	return &Deps{Sender: sender, State: stateMgr, CatRepo: greetingsRepo(), PromptRepo: prompts}
}

func TestGreetingsMenuShowsAddressees(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	fc := &Context{
		VkID:     401,
		User:     &User{Gender: "unknown", FreeGens: 1},
		State:    &State{Step: StepMainMenu},
		Callback: &CallbackData{Type: "greetings"},
	}

	HandleGreetingsMenu(context.Background(), fc, greetingsDeps(sender, stateMgr, &fakePromptRepo{}))

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "greetings_intro" {
		t.Fatalf("expected the greetings_intro screen, got %q", last.Key)
	}
	keyboard := decodeKeyboard(t, last.Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "Мужчине" {
		t.Fatalf("expected the first addressee, got %q", got)
	}
	if got := keyboard.Buttons[1][0].Action.Label; got != "Женщине" {
		t.Fatalf("expected the second addressee, got %q", got)
	}
}

func TestGreetingsAddresseeShowsHolidaysOnItsOwnScreen(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	fc := &Context{
		VkID:     402,
		User:     &User{Gender: "female", FreeGens: 1},
		State:    &State{Step: StepGreetingsCategories, Section: repository.SectionGreetings, PromptType: "greetings"},
		Callback: &CallbackData{CategoryID: 70},
	}

	HandleOpenCategory(context.Background(), fc, greetingsDeps(sender, stateMgr, &fakePromptRepo{}))

	last := sender.screens[len(sender.screens)-1]
	// Второй уровень живёт на своём экране: у адресата задан screen_key.
	if last.Key != "greetings_holidays" {
		t.Fatalf("expected the greetings_holidays screen, got %q", last.Key)
	}
	keyboard := decodeKeyboard(t, last.Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "День рождения" {
		t.Fatalf("expected the first holiday, got %q", got)
	}
	state := stateMgr.states[402]
	if state == nil || state.SectionID != 70 {
		t.Fatalf("expected the addressee to be remembered, got %#v", state)
	}
}

func TestGreetingsHolidayInheritsGenderFromAddressee(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	prompts := &fakePromptRepo{
		byCategoryGender: map[string][]*repository.Prompt{
			// Пол у праздника не задан: он должен прийти от адресата «Мужчине».
			promptKey(72, "male"): makePrompts(72, 3),
		},
	}
	fc := &Context{
		VkID:     403,
		User:     &User{Gender: "female", FreeGens: 1},
		State:    &State{Step: StepGreetingsCategories, Section: repository.SectionGreetings, PromptType: "greetings", SectionID: 70},
		Callback: &CallbackData{CategoryID: 72},
	}

	HandleOpenCategory(context.Background(), fc, greetingsDeps(sender, stateMgr, prompts))

	if got := sender.screens[len(sender.screens)-1].Key; got != "prompts_list" {
		t.Fatalf("expected the prompt list, got %q", got)
	}
	state := stateMgr.states[403]
	if state == nil || state.Step != StepGreetingsPrompts {
		t.Fatalf("expected %q step, got %#v", StepGreetingsPrompts, state)
	}
	if state.SectionID != 70 {
		t.Fatalf("expected the holiday's parent in state, got %d", state.SectionID)
	}
}

func TestGreetingsBackWalksThreeLevels(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	prompts := &fakePromptRepo{
		byCategoryGender: map[string][]*repository.Prompt{promptKey(72, "male"): makePrompts(72, 2)},
	}
	deps := greetingsDeps(sender, stateMgr, prompts)
	fc := &Context{
		VkID:     404,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepGreetingsCategories, Section: repository.SectionGreetings, PromptType: "greetings", SectionID: 70},
		Callback: &CallbackData{CategoryID: 72},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	// С промтов — обратно к праздникам.
	fc.Callback = &CallbackData{Type: "back"}
	HandleBack(context.Background(), fc, deps)
	if got := sender.screens[len(sender.screens)-1].Key; got != "greetings_holidays" {
		t.Fatalf("expected to return to the holidays, got %q", got)
	}

	// С праздников — к адресатам.
	HandleBack(context.Background(), fc, deps)
	if got := sender.screens[len(sender.screens)-1].Key; got != "greetings_intro" {
		t.Fatalf("expected to return to the addressees, got %q", got)
	}

	// С адресатов — наружу, в главное меню: навигатор больше не обрабатывает.
	if sectionBack(context.Background(), fc, deps) {
		t.Fatal("expected the root level to fall through to the main menu")
	}
}

func TestGreetingsHolidayWithoutPromptsShowsSoonScreen(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	fc := &Context{
		VkID:     405,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepGreetingsCategories, Section: repository.SectionGreetings, PromptType: "greetings", SectionID: 70},
		Callback: &CallbackData{CategoryID: 73},
	}

	HandleOpenCategory(context.Background(), fc, greetingsDeps(sender, stateMgr, &fakePromptRepo{}))

	if got := sender.screens[len(sender.screens)-1].Key; got != "section_soon" {
		t.Fatalf("expected the section_soon screen, got %q", got)
	}
}

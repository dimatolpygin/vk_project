package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

// trendsRepo — раздел «Тренды» после миграции 00033 в том виде, в каком его
// отдаёт ListRoots: три видео-узла лежат с is_active = false, поэтому до
// навигатора доезжает только «Фото тренды».
func trendsRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{
		trends: []*repository.Category{
			{ID: 80, Name: "📸 Фото тренды", Section: repository.SectionTrends, MediaKind: repository.MediaKindPhoto},
		},
	}
}

func TestTrendsMenuShowsOnlyPhotoNode(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: trendsRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     401,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepMainMenu},
		Callback: &CallbackData{Type: "trends"},
	}

	HandleTrendsMenu(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "trends_intro" {
		t.Fatalf("expected the trends_intro screen, got %q", last.Key)
	}
	keyboard := decodeKeyboard(t, last.Keyboard)
	if got := keyboard.Buttons[0][0].Action.Label; got != "📸 Фото тренды" {
		t.Fatalf("expected the photo trends node first, got %q", got)
	}
	// Кнопок, ведущих в невыпущенную видео-генерацию, у пользователя быть не должно.
	for _, row := range keyboard.Buttons {
		for _, btn := range row {
			if btn.Action.Label == "🎬 Видео тренды" || btn.Action.Label == "✨ Оживить фото" || btn.Action.Label == "💃 Танцы по фото" {
				t.Fatalf("video node leaked into the menu: %q", btn.Action.Label)
			}
		}
	}
	state := stateMgr.states[401]
	if state == nil || state.Section != repository.SectionTrends {
		t.Fatalf("expected the trends section in state, got %#v", state)
	}
	if state.Step != StepTrendsCategories {
		t.Fatalf("expected %q step, got %q", StepTrendsCategories, state.Step)
	}
	if state.PromptType != "trends" {
		t.Fatalf("expected the trends prompt type, got %q", state.PromptType)
	}
}

func TestTrendsPromptsUseUserGender(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{
		Sender:  sender,
		State:   stateMgr,
		CatRepo: trendsRepo(),
		PromptRepo: &fakePromptRepo{
			byCategoryGender: map[string][]*repository.Prompt{
				// В отличие от детского раздела и поздравлений, пол на узле не задан —
				// значит, отбор идёт по полу самого пользователя.
				promptKey(80, "female"): makePrompts(80, 3),
			},
		},
	}
	fc := &Context{
		VkID:     402,
		User:     &User{Gender: "female", FreeGens: 1},
		State:    &State{Step: StepTrendsCategories, Section: repository.SectionTrends, PromptType: "trends"},
		Callback: &CallbackData{CategoryID: 80},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "prompts_list" {
		t.Fatalf("expected the prompt list, got %q", got)
	}
	state := stateMgr.states[402]
	if state == nil || state.Step != StepTrendsPrompts {
		t.Fatalf("expected %q step, got %#v", StepTrendsPrompts, state)
	}
}

func TestEmptyTrendsNodeShowsSoonScreen(t *testing.T) {
	// Раздел выкатывается пустым: промты заказчик заводит из админки, а до тех пор
	// узел обязан объяснить это словами, а не пустым списком.
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: trendsRepo(), PromptRepo: &fakePromptRepo{}}
	fc := &Context{
		VkID:     403,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepTrendsCategories, Section: repository.SectionTrends, PromptType: "trends"},
		Callback: &CallbackData{CategoryID: 80},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "section_soon" {
		t.Fatalf("expected the section_soon screen, got %q", got)
	}
}

func TestBackFromTrendsWalksToSectionRootThenOut(t *testing.T) {
	if !IsSectionStep(StepTrendsCategories) || !IsSectionStep(StepTrendsPrompts) {
		t.Fatal("expected the trends steps to be recognised as section steps")
	}
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: trendsRepo(), PromptRepo: &fakePromptRepo{}}

	// С промтов «Назад» возвращает на корень раздела, а не в главное меню.
	fc := &Context{
		VkID:     404,
		User:     &User{Gender: "male", FreeGens: 1},
		State:    &State{Step: StepTrendsPrompts, Section: repository.SectionTrends, PromptType: "trends", CategoryID: 80},
		Callback: &CallbackData{Type: "back"},
	}
	if !sectionBack(context.Background(), fc, deps) {
		t.Fatal("expected the back button to be handled inside the section")
	}
	if got := sender.screens[len(sender.screens)-1].Key; got != "trends_intro" {
		t.Fatalf("expected a return to the section root, got %q", got)
	}

	// А уже с корня — наружу, в главное меню.
	if sectionBack(context.Background(), fc, deps) {
		t.Fatal("expected the root level to fall through to the main menu")
	}
}

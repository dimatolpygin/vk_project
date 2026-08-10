package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

// Подраздел трендов показывает промты сразу — детей у него нет. До этапа 12 это
// был единственный шаг, где узел не мог показать свою картинку и описание:
// screen_key спрашивался только у узлов с подменю.
func TestPromptsListUsesNodeScreenWhenSet(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	cats := &fakeCategoryRepo{
		trends: []*repository.Category{
			{ID: 81, Name: "🎬 Видео тренды", Section: repository.SectionTrends, PromptsScreenKey: strptr("node_81_prompts")},
		},
	}
	prompts := &fakePromptRepo{byCategoryGender: map[string][]*repository.Prompt{
		promptKey(81, "male"): {{ID: 900, Name: "Тренд", Prompt: "p"}},
	}}
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: cats, PromptRepo: prompts}
	fc := &Context{
		VkID:     501,
		User:     &User{Gender: "male", FreeGens: 5},
		State:    &State{Step: StepTrendsCategories, Section: repository.SectionTrends},
		Callback: &CallbackData{Type: "open_category", CategoryID: 81},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "node_81_prompts" {
		t.Fatalf("expected the node screen above the prompt list, got %q", last.Key)
	}
}

// Узел без своих экранов ведёт себя ровно как до этапа: общий список.
func TestPromptsListFallsBackToSharedScreen(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	cats := &fakeCategoryRepo{
		trends: []*repository.Category{
			{ID: 82, Name: "📸 Фото тренды", Section: repository.SectionTrends},
		},
	}
	prompts := &fakePromptRepo{byCategoryGender: map[string][]*repository.Prompt{
		promptKey(82, "male"): {{ID: 901, Name: "Тренд", Prompt: "p"}},
	}}
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: cats, PromptRepo: prompts}
	fc := &Context{
		VkID:     502,
		User:     &User{Gender: "male", FreeGens: 5},
		State:    &State{Step: StepTrendsCategories, Section: repository.SectionTrends},
		Callback: &CallbackData{Type: "open_category", CategoryID: 82},
	}

	HandleOpenCategory(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "prompts_list" {
		t.Fatalf("expected the shared prompt list screen, got %q", last.Key)
	}
}

// Детский раздел: текст запроса фото задан на узле «Мальчик», а промт лежит
// в его подкатегории — без наследования настройка не доехала бы до шага,
// на котором нужна.
func TestPhotoRequestScreenInheritsFromParentNode(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	boyID := 70
	cats := &fakeCategoryRepo{
		kids: []*repository.Category{
			{ID: boyID, Name: "👦 Мальчик", Section: repository.SectionKids,
				PromptGender: strptr("male"), PhotoScreenKey: strptr("node_70_photo")},
		},
		children: map[int][]*repository.Category{
			boyID: {{ID: 71, Name: "Спорт", Section: repository.SectionKids, ParentID: &boyID}},
		},
	}
	prompts := &fakePromptRepo{byID: map[int]*repository.Prompt{
		910: {ID: 910, Name: "Футболист", Prompt: "p", CategoryID: 71},
	}}
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: cats, PromptRepo: prompts}
	fc := &Context{
		VkID:     503,
		User:     &User{Gender: "female", FreeGens: 5},
		State:    &State{Step: StepKidsPrompts, Section: repository.SectionKids, PromptType: "kids", CategoryID: 71},
		Callback: &CallbackData{Type: "select_prompt", PromptID: 910},
	}

	HandleSelectPrompt(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "node_70_photo" {
		t.Fatalf("expected the node photo screen inherited from the parent, got %q", last.Key)
	}
}

// Разделы без своих экранов просят фото прежним общим текстом.
func TestPhotoRequestScreenFallsBackToShared(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	cats := &fakeCategoryRepo{
		readyByGender: map[string][]*repository.Category{
			"male": {{ID: 10, Name: "Деловой стиль"}},
		},
	}
	prompts := &fakePromptRepo{byID: map[int]*repository.Prompt{
		911: {ID: 911, Name: "Костюм", Prompt: "p", CategoryID: 10},
	}}
	deps := &Deps{Sender: sender, State: stateMgr, CatRepo: cats, PromptRepo: prompts}
	fc := &Context{
		VkID:     504,
		User:     &User{Gender: "male", FreeGens: 5},
		State:    &State{Step: StepReadyPromptsPrompts, Section: repository.SectionSelf, PromptType: "ready_prompt", CategoryID: 10},
		Callback: &CallbackData{Type: "select_prompt", PromptID: 911},
	}

	HandleSelectPrompt(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "photo_requirements" {
		t.Fatalf("expected the shared photo screen, got %q", last.Key)
	}
}

package flows

import (
	"context"
	"testing"

	"vk_neuro_bot/internal/repository"
)

// videoPrompt — тренд из «Видео трендов» после этапа 10: два промта в одной
// карточке и цена в генерациях.
func videoPrompt() *repository.Prompt {
	return &repository.Prompt{
		ID:          501,
		CategoryID:  81,
		Name:        "Танец у окна",
		Prompt:      "cinematic portrait by the window, golden hour",
		VideoPrompt: "slow camera push in, model turns to the camera",
		MediaKind:   repository.MediaKindVideo,
		PriceGens:   40,
	}
}

func videoDeps(sender *fakeSender, stateMgr *fakeStateMgr) *Deps {
	prompt := videoPrompt()
	return &Deps{
		Sender:  sender,
		State:   stateMgr,
		CatRepo: trendsRepo(),
		PromptRepo: &fakePromptRepo{
			byID: map[int]*repository.Prompt{prompt.ID: prompt},
			byCategoryGender: map[string][]*repository.Prompt{
				promptKey(81, "female"): {prompt},
			},
		},
	}
}

func TestVideoPromptRefusedWhenBalanceIsShort(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := videoDeps(sender, stateMgr)
	fc := &Context{
		VkID:     501,
		User:     &User{Gender: "female", FreeGens: 3, PaidGens: 12},
		State:    &State{Step: StepTrendsPrompts, Section: repository.SectionTrends, PromptType: "trends", CategoryID: 81},
		Callback: &CallbackData{PromptID: 501},
	}

	HandleSelectPrompt(context.Background(), fc, deps)

	last := sender.screens[len(sender.screens)-1]
	if last.Key != "no_gens_for_video" {
		t.Fatalf("ожидался экран нехватки генераций, получен %q", last.Key)
	}
	// Фото у пользователя не просим: платить всё равно нечем.
	if st := stateMgr.states[501]; st != nil && st.Step == StepAwaitingPhoto {
		t.Fatal("бот попросил фото, хотя генераций не хватает")
	}
}

func TestNotEnoughGensDataCountsBothBalances(t *testing.T) {
	// 12 платных и 3 бесплатных складываются: не хватает 25, а не 28 и не 37.
	data := notEnoughGensData(&User{FreeGens: 3, PaidGens: 12}, 40)
	if data["CostGens"] != 40 {
		t.Fatalf("цена в экране: %v, ожидалось 40", data["CostGens"])
	}
	if data["UserGens"] != 15 {
		t.Fatalf("баланс в экране: %v, ожидалось 15", data["UserGens"])
	}
	if data["MissingGens"] != 25 {
		t.Fatalf("нехватка в экране: %v, ожидалось 25", data["MissingGens"])
	}

	// Хватает ровно впритык — отказа быть не должно, а нехватка не уходит в минус.
	if !hasGensFor(&User{FreeGens: 40}, 40) {
		t.Fatal("баланс ровно в цену признан недостаточным")
	}
	if got := notEnoughGensData(&User{FreeGens: 41}, 40)["MissingGens"]; got != 0 {
		t.Fatalf("нехватка при достаточном балансе: %v, ожидался 0", got)
	}
}

func TestVideoPromptAsksForPhotoWhenBalanceIsEnough(t *testing.T) {
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	deps := videoDeps(sender, stateMgr)
	fc := &Context{
		VkID:     502,
		User:     &User{Gender: "female", PaidGens: 40},
		State:    &State{Step: StepTrendsPrompts, Section: repository.SectionTrends, PromptType: "trends", CategoryID: 81},
		Callback: &CallbackData{PromptID: 501},
	}

	HandleSelectPrompt(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "photo_requirements" {
		t.Fatalf("ожидался запрос фото, получен экран %q", got)
	}
	st := stateMgr.states[502]
	if st == nil || st.Step != StepAwaitingPhoto {
		t.Fatalf("ожидался шаг ожидания фото, получено %#v", st)
	}
	// Шаблон запоминается: без него после загрузки фото не с чем идти в цепочку.
	if st.TemplateID != 501 {
		t.Fatalf("шаблон не сохранён в состоянии: %d", st.TemplateID)
	}
	// Раздел не теряется, иначе «Назад» уводит из трендов в главное меню.
	if st.Section != repository.SectionTrends || st.CategoryID != 81 {
		t.Fatalf("раздел потерян: section=%q category=%d", st.Section, st.CategoryID)
	}
}

func TestPhotoPromptStillGoesThroughPhotoFlow(t *testing.T) {
	// Соседний фото-промт в том же разделе не должен зацепить видео-ветку.
	sender := &fakeSender{}
	stateMgr := newFakeStateMgr()
	photo := &repository.Prompt{ID: 502, CategoryID: 80, Name: "Обычный тренд", Prompt: "portrait"}
	deps := &Deps{
		Sender:     sender,
		State:      stateMgr,
		CatRepo:    trendsRepo(),
		PromptRepo: &fakePromptRepo{byID: map[int]*repository.Prompt{502: photo}},
	}
	fc := &Context{
		VkID:     503,
		User:     &User{Gender: "female", FreeGens: 1},
		State:    &State{Step: StepTrendsPrompts, Section: repository.SectionTrends, PromptType: "trends", CategoryID: 80},
		Callback: &CallbackData{PromptID: 502},
	}

	HandleSelectPrompt(context.Background(), fc, deps)

	if got := sender.screens[len(sender.screens)-1].Key; got != "photo_requirements" {
		t.Fatalf("ожидался запрос фото, получен экран %q", got)
	}
	// Одной генерации хватает на фото — экрана про видео здесь быть не должно.
	for _, screen := range sender.screens {
		if screen.Key == "no_gens_for_video" {
			t.Fatal("фото-промт ушёл в проверку цены видео")
		}
	}
}

package flows

import "context"

func HandleReadyPromptsMenu(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
		return
	}
	cats, err := d.CatRepo.ListActive(ctx, fc.User.Gender)
	if err != nil || len(cats) == 0 {
		_ = d.Sender.SendText(ctx, fc.VkID, "Категории пока не добавлены.", KbBack())
		return
	}
	_ = d.Sender.SendMsg(ctx, fc.VkID, "ready_prompts_intro", KbCategories(cats))
}

func HandleSelectCategory(ctx context.Context, fc *Context, d *Deps) {
	catID := fc.Callback.CategoryID
	promptType := "ready_prompt"
	gender := fc.User.Gender
	if fc.State.PromptType == "couple" {
		promptType = "couple"
		gender = "couple"
	}
	prompts, err := d.PromptRepo.ListByCategory(ctx, catID, gender)
	if err != nil || len(prompts) == 0 {
		_ = d.Sender.SendText(ctx, fc.VkID, "В этой категории пока нет шаблонов.", KbBack())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhoto, PromptType: promptType, CategoryID: catID}, fc.State))
	_ = d.Sender.SendText(ctx, fc.VkID, "Выбери стиль:", KbPrompts(prompts))
}

func HandleSelectPrompt(ctx context.Context, fc *Context, d *Deps) {
	promptID := fc.Callback.PromptID
	p, err := d.PromptRepo.GetByID(ctx, promptID)
	if err != nil || p == nil {
		_ = d.Sender.SendText(ctx, fc.VkID, "Шаблон не найден.", KbBack())
		return
	}
	promptType := fc.State.PromptType
	if promptType == "" {
		promptType = "ready_prompt"
	}
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{
		Step:       StepAwaitingPhoto,
		PromptType: promptType,
		TemplateID: promptID,
	}, fc.State))
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
}

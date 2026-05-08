package flows

import "context"

func HandleReadyPromptsMenu(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	categories, err := d.CatRepo.ListActive(ctx, fc.User.Gender)
	if err != nil || len(categories) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "categories_empty", ScreenOptions{})
		return
	}

	_ = sendScreen(ctx, d, fc.VkID, "ready_prompts_intro", ScreenOptions{PrefixRows: categoryRows(categories)})
}

func HandleSelectCategory(ctx context.Context, fc *Context, d *Deps) {
	categoryID := fc.Callback.CategoryID
	promptType := "ready_prompt"
	gender := fc.User.Gender
	if fc.State.PromptType == "couple" {
		promptType = "couple"
		gender = "couple"
	}

	prompts, err := d.PromptRepo.ListByCategory(ctx, categoryID, gender)
	if err != nil || len(prompts) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "prompts_empty", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhoto, PromptType: promptType, CategoryID: categoryID}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "prompts_list", ScreenOptions{PrefixRows: promptRows(prompts)})
}

func HandleSelectPrompt(ctx context.Context, fc *Context, d *Deps) {
	promptID := fc.Callback.PromptID
	prompt, err := d.PromptRepo.GetByID(ctx, promptID)
	if err != nil || prompt == nil {
		_ = sendScreen(ctx, d, fc.VkID, "prompt_not_found", ScreenOptions{})
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

	if fc.User.UseSavedPhoto && fc.User.SavedPhotoURL != nil && *fc.User.SavedPhotoURL != "" {
		if !fc.User.HasGens() {
			_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
			return
		}
		startGeneration(ctx, fc, d, *fc.User.SavedPhotoURL, prompt.Prompt, promptType, "saved_photo_generation_wait", map[string]any{
			"PromptName": prompt.Name,
		})
		return
	}

	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

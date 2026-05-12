package flows

import (
	"context"

	"github.com/rs/zerolog/log"
)

func HandleReadyPromptsMenu(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	if err := showReadyPromptsCategoryPage(ctx, fc, d, 1); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка отправки страницы готовых промтов")
	}
}

func HandleReadyPromptsPage(ctx context.Context, fc *Context, d *Deps) {
	page := normalizePage(fc.Callback.Page)
	if err := showReadyPromptsCategoryPage(ctx, fc, d, page); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", page).Msg("ошибка отправки страницы категорий")
	}
}

func HandlePromptsPage(ctx context.Context, fc *Context, d *Deps) {
	categoryID := fc.Callback.CategoryID
	if categoryID == 0 {
		categoryID = fc.State.CategoryID
	}

	categoryPage := normalizePage(fc.State.CategoryPage)
	promptPage := normalizePage(fc.Callback.Page)
	if categoryID == 0 {
		if fc.State.PromptType == "couple" {
			if err := showCoupleCategoryPage(ctx, fc, d, categoryPage); err != nil {
				log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", categoryPage).Msg("ошибка возврата к paginated couple категориям")
			}
			return
		}
		if err := showReadyPromptsCategoryPage(ctx, fc, d, categoryPage); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", categoryPage).Msg("ошибка возврата к paginated ready_prompts категориям")
		}
		return
	}

	if err := showPromptPage(ctx, fc, d, categoryID, categoryPage, promptPage); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", categoryID).Int("page", promptPage).Msg("ошибка отправки страницы шаблонов")
	}
}

func HandleSelectCategory(ctx context.Context, fc *Context, d *Deps) {
	categoryID := fc.Callback.CategoryID
	promptType := "ready_prompt"
	gender := fc.User.Gender
	step := StepReadyPromptsPrompts
	if fc.State.PromptType == "couple" || fc.State.Step == StepCoupleCategories || fc.State.Step == StepCouplePrompts {
		promptType = "couple"
		gender = "couple"
		step = StepCouplePrompts
	}

	prompts, err := d.PromptRepo.ListByCategory(ctx, categoryID, gender)
	if err != nil || len(prompts) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "prompts_empty", ScreenOptions{})
		return
	}

	rows, promptPage, _ := buildPaginatedRows(
		promptButtons(prompts),
		1,
		"prompts_page",
		map[string]any{"category_id": categoryID},
	)
	state := copyPrefs(&State{
		Step:         step,
		PromptType:   promptType,
		CategoryID:   categoryID,
		CategoryPage: normalizePage(fc.State.CategoryPage),
		PromptPage:   promptPage,
	}, fc.State)
	_ = d.State.Set(ctx, fc.VkID, state)
	if err := sendScreen(ctx, d, fc.VkID, "prompts_list", ScreenOptions{PrefixRows: rows}); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", categoryID).Msg("ошибка отправки первой страницы шаблонов")
	}
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
		Step:         StepAwaitingPhoto,
		PromptType:   promptType,
		TemplateID:   promptID,
		CategoryID:   fc.State.CategoryID,
		CategoryPage: fc.State.CategoryPage,
		PromptPage:   fc.State.PromptPage,
	}, fc.State))

	if fc.User.UseSavedPhoto && len(fc.User.SavedPhotoURLs) > 0 {
		if !fc.User.HasGens() {
			_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
			return
		}
		startGeneration(ctx, fc, d, fc.User.SavedPhotoURLs, prompt.Prompt, promptType, "saved_photo_generation_wait", map[string]any{
			"PromptName": prompt.Name,
		})
		return
	}

	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleReadyPromptsBrowse(ctx context.Context, fc *Context, d *Deps) {
	switch fc.State.Step {
	case StepReadyPromptsCategories:
		if err := showReadyPromptsCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("ошибка повторной отправки ready_prompts категорий")
		}
	case StepReadyPromptsPrompts:
		if fc.State.CategoryID == 0 {
			if err := showReadyPromptsCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
				log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("ошибка возврата к ready_prompts категориям")
			}
			return
		}
		if err := showPromptPage(ctx, fc, d, fc.State.CategoryID, normalizePage(fc.State.CategoryPage), normalizePage(fc.State.PromptPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", fc.State.CategoryID).Int("page", fc.State.PromptPage).Msg("ошибка повторной отправки страницы шаблонов")
		}
	}
}

func showReadyPromptsCategoryPage(ctx context.Context, fc *Context, d *Deps, page int) error {
	if fc.User.Gender == "unknown" {
		_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{
			Step:       StepAwaitingGender,
			PromptType: "ready_prompt",
		}, fc.State))
		return sendScreen(ctx, d, fc.VkID, "gender_select", ScreenOptions{})
	}

	categories, err := d.CatRepo.ListReadyPromptCategories(ctx, fc.User.Gender)
	if err != nil || len(categories) == 0 {
		return sendScreen(ctx, d, fc.VkID, "categories_empty", ScreenOptions{})
	}

	rows, currentPage, _ := buildPaginatedRows(categoryButtons(categories), page, "ready_prompts_page", nil)
	state := copyPrefs(&State{
		Step:         StepReadyPromptsCategories,
		PromptType:   "ready_prompt",
		CategoryPage: currentPage,
		PromptPage:   1,
	}, fc.State)
	_ = d.State.Set(ctx, fc.VkID, state)
	return sendScreen(ctx, d, fc.VkID, "ready_prompts_intro", ScreenOptions{PrefixRows: rows})
}

func showPromptPage(ctx context.Context, fc *Context, d *Deps, categoryID, categoryPage, promptPage int) error {
	promptType := "ready_prompt"
	gender := fc.User.Gender
	step := StepReadyPromptsPrompts
	if fc.State.PromptType == "couple" || fc.State.Step == StepCoupleCategories || fc.State.Step == StepCouplePrompts {
		promptType = "couple"
		gender = "couple"
		step = StepCouplePrompts
	}

	prompts, err := d.PromptRepo.ListByCategory(ctx, categoryID, gender)
	if err != nil || len(prompts) == 0 {
		return sendScreen(ctx, d, fc.VkID, "prompts_empty", ScreenOptions{})
	}

	rows, currentPage, _ := buildPaginatedRows(
		promptButtons(prompts),
		promptPage,
		"prompts_page",
		map[string]any{"category_id": categoryID},
	)
	state := copyPrefs(&State{
		Step:         step,
		PromptType:   promptType,
		CategoryID:   categoryID,
		CategoryPage: normalizePage(categoryPage),
		PromptPage:   currentPage,
	}, fc.State)
	_ = d.State.Set(ctx, fc.VkID, state)
	return sendScreen(ctx, d, fc.VkID, "prompts_list", ScreenOptions{PrefixRows: rows})
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

package flows

import (
	"context"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
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

// HandleSelectCategory — старый callback выбора категории. Оставлен алиасом
// навигатора: клавиатуры с ним уже разосланы пользователям.
func HandleSelectCategory(ctx context.Context, fc *Context, d *Deps) {
	HandleOpenCategory(ctx, fc, d)
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

	// Видео-тренд стоит десятки генераций, поэтому у него своя ветка: цена
	// проверяется до того, как у пользователя попросят фото.
	if prompt.IsVideo() {
		handleVideoPromptSelected(ctx, fc, d, prompt, promptType)
		return
	}

	// Парный режим: фото уже загружены до выбора категории — запускаем генерацию сразу.
	if promptType == "couple" {
		couplePhotos := normalizeGenerationInputPhotos(fc.State.CouplePhotoURLs)
		if len(couplePhotos) == 0 {
			// Страховка: фото просят на входе в режим, но клавиатура из старой
			// переписки может привести сюда в обход этого шага.
			cat, err := d.CatRepo.GetByID(ctx, fc.State.CategoryID)
			if err != nil || cat == nil {
				cat = &repository.Category{ID: fc.State.CategoryID, Section: repository.SectionCouple}
			}
			if err := askCouplePhoto(ctx, fc, d, cat); err != nil {
				log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка запроса парного фото")
			}
			return
		}
		if !fc.User.HasGens() {
			_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
			return
		}

		genState := copyPrefs(&State{
			Step:         StepCouplePrompts,
			PromptType:   "couple",
			TemplateID:   promptID,
			CategoryID:   fc.State.CategoryID,
			CategoryPage: fc.State.CategoryPage,
			PromptPage:   fc.State.PromptPage,
		}, fc.State)
		genState.CouplePhotoURLs = couplePhotos
		fc.State = genState
		_ = d.State.Set(ctx, fc.VkID, genState)

		startGeneration(ctx, fc, d, couplePhotos, prompt.Prompt, "couple", "generating_wait", nil)
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{
		Step:         StepAwaitingPhoto,
		PromptType:   promptType,
		TemplateID:   promptID,
		CategoryID:   fc.State.CategoryID,
		CategoryPage: fc.State.CategoryPage,
		PromptPage:   fc.State.PromptPage,
	}, fc.State))

	// «Запомнить фото» хранит фото самого пользователя, а детскому разделу нужно
	// фото ребёнка — там всегда просим новое.
	if fc.User.UseSavedPhoto && len(fc.User.SavedPhotoURLs) > 0 && fc.State.Section != repository.SectionKids {
		if !fc.User.HasGens() {
			_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
			return
		}
		startGeneration(ctx, fc, d, fc.User.SavedPhotoURLs, prompt.Prompt, promptType, "saved_photo_generation_wait", map[string]any{
			"PromptName": prompt.Name,
		})
		return
	}

	// Детский раздел просит фото ребёнка, а не пользователя, и текст у «Мальчика»
	// и «Девочки» разный — экран берётся с узла, если админ его задал.
	_ = sendScreen(ctx, d, fc.VkID, photoRequestScreen(ctx, fc, d), ScreenOptions{})
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
	// Пол нужен раньше категорий: в разделе «Фото для себя» промты отбираются
	// по полу пользователя.
	if fc.User.Gender == "unknown" {
		_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{
			Step:       StepAwaitingGender,
			PromptType: "ready_prompt",
		}, fc.State))
		return sendScreen(ctx, d, fc.VkID, "gender_select", ScreenOptions{})
	}

	return showSectionRoots(ctx, fc, d, specForSection(repository.SectionSelf), page)
}

func showPromptPage(ctx context.Context, fc *Context, d *Deps, categoryID, categoryPage, promptPage int) error {
	cat, err := d.CatRepo.GetByID(ctx, categoryID)
	if err != nil {
		log.Error().Err(err).Int("category_id", categoryID).Msg("ошибка получения категории для списка шаблонов")
	}

	// Узел мог быть удалён из админки между отправкой клавиатуры и нажатием —
	// тогда опираемся на раздел из состояния пользователя.
	spec := specForState(fc)
	gender := sectionGender(spec, fc)
	if cat != nil {
		spec = specForSection(cat.Section)
		gender = resolveNodeGender(ctx, d, cat, fc)
	}

	prompts, err := d.PromptRepo.ListByCategory(ctx, categoryID, gender)
	if err != nil || len(prompts) == 0 {
		if err != nil {
			log.Error().Err(err).Int("category_id", categoryID).Msg("ошибка получения шаблонов категории")
			return sendScreen(ctx, d, fc.VkID, "prompts_empty", ScreenOptions{})
		}
		// Узел заведён, но ещё не наполнен — показываем «Скоро», а не сухое
		// «шаблонов нет». Состояние пишем как для обычного списка промтов,
		// иначе «Назад» уводит в главное меню мимо уровня, с которого пришли.
		emptyState := copyPrefs(&State{
			Step:         spec.promptsStep,
			PromptType:   spec.promptType,
			Section:      spec.section,
			SectionID:    parentNodeID(cat),
			CategoryID:   categoryID,
			CategoryPage: normalizePage(categoryPage),
			PromptPage:   1,
		}, fc.State)
		fc.State = emptyState
		_ = d.State.Set(ctx, fc.VkID, emptyState)
		return sendScreen(ctx, d, fc.VkID, "section_soon", ScreenOptions{})
	}

	rows, currentPage, _ := buildPaginatedRows(
		promptButtons(prompts),
		promptPage,
		"prompts_page",
		map[string]any{"category_id": categoryID},
	)
	state := copyPrefs(&State{
		Step:         spec.promptsStep,
		PromptType:   spec.promptType,
		Section:      spec.section,
		SectionID:    parentNodeID(cat),
		CategoryID:   categoryID,
		CategoryPage: normalizePage(categoryPage),
		PromptPage:   currentPage,
	}, fc.State)
	fc.State = state
	_ = d.State.Set(ctx, fc.VkID, state)
	// У подраздела трендов детей нет, промты показываются сразу — и до этапа 12
	// это был единственный шаг, где узел не мог сказать ничего своего.
	return sendScreen(ctx, d, fc.VkID, nodeStepScreen(ctx, d, cat, promptsScreenOf, "prompts_list"), ScreenOptions{PrefixRows: rows})
}

// parentNodeID — уровень, с которого пользователь пришёл в категорию: 0 означает
// корень раздела.
func parentNodeID(cat *repository.Category) int {
	if cat == nil || cat.ParentID == nil {
		return 0
	}
	return *cat.ParentID
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

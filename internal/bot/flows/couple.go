package flows

import (
	"context"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

// HandleCoupleStart — вход в раздел. С этапа 13 порядок обратный прежнему:
// сначала пользователь выбирает режим съёмки и видит, ради чего он отдаёт фото,
// и только потом его просят прислать снимок.
func HandleCoupleStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	// Фото прошлого захода не переносим: пользователь мог вернуться в раздел
	// именно затем, чтобы прислать другое.
	state := copyPrefs(&State{
		Step:       StepCoupleCategories,
		PromptType: "couple",
		Section:    repository.SectionCouple,
	}, fc.State)
	state.CouplePhotoURLs = nil
	state.InputPhotoURLs = nil
	state.PhotoBatchID = ""
	fc.State = state
	_ = d.State.Set(ctx, fc.VkID, state)

	if err := showCoupleCategoryPage(ctx, fc, d, 1); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка показа режимов парной съёмки")
	}
}

// askCouplePhoto просит фото на входе в режим съёмки. Экран берётся с узла,
// если админ задал свой: у «Парного», «Семейного» и «Поколений» разные картинки
// и описания — ради этого шаг и переехал сюда.
func askCouplePhoto(ctx context.Context, fc *Context, d *Deps, cat *repository.Category) error {
	state := copyPrefs(&State{
		Step:         StepCoupleAwaitingPhoto,
		PromptType:   "couple",
		Section:      repository.SectionCouple,
		SectionID:    parentNodeID(cat),
		CategoryID:   cat.ID,
		CategoryPage: normalizePage(fc.State.CategoryPage),
		PromptPage:   1,
	}, fc.State)
	state.CouplePhotoURLs = nil
	state.InputPhotoURLs = nil
	state.PhotoBatchID = ""
	fc.State = state
	_ = d.State.Set(ctx, fc.VkID, state)

	return sendScreen(ctx, d, fc.VkID, nodeStepScreen(ctx, d, cat, photoScreenOf, "couple_intro"), ScreenOptions{})
}

// HandleCoupleAwaitingPhoto принимает фото пары/семьи и только после успешной
// загрузки показывает содержимое выбранного режима.
func HandleCoupleAwaitingPhoto(ctx context.Context, fc *Context, d *Deps) {
	photos := normalizeGenerationInputPhotos(messagePhotos(fc.Message))
	log.Info().Int64("vk_id", fc.VkID).Int("message_photo_count", len(photos)).Msg("couple: фото во входящем сообщении")

	// Экран повтора — тот же, которым просили фото: пользователь прислал текст
	// вместо снимка, и подсовывать ему другой текст незачем.
	repeatScreen := coupleAwaitingScreen(ctx, fc, d)

	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, repeatScreen, ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	batchID, ownsBatch := appendPendingPhotoBatch(ctx, fc, d, photos)
	if !ownsBatch {
		return
	}

	batchPhotos := photos
	if batchID != "" {
		pendingState, pendingPhotos, ok := waitForPendingPhotoBatch(ctx, d, fc.VkID, batchID)
		if !ok {
			return
		}
		fc.State = pendingState
		batchPhotos = pendingPhotos
	}

	uploadedURLs := uploadGenerationInputPhotos(ctx, d, fc.VkID, batchPhotos, "couple_upload")
	if len(uploadedURLs) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, repeatScreen, ScreenOptions{})
		return
	}

	nodeID := fc.State.CategoryID
	carried := copyPrefs(&State{
		Step:         StepCoupleCategories,
		PromptType:   "couple",
		Section:      repository.SectionCouple,
		SectionID:    fc.State.SectionID,
		CategoryID:   nodeID,
		CategoryPage: normalizePage(fc.State.CategoryPage),
		PromptPage:   1,
	}, fc.State)
	carried.CouplePhotoURLs = uploadedURLs
	carried.InputPhotoURLs = nil
	carried.PhotoBatchID = ""
	fc.State = carried
	_ = d.State.Set(ctx, fc.VkID, carried)

	log.Info().Int64("vk_id", fc.VkID).Int("photo_count", len(uploadedURLs)).Int("category_id", nodeID).
		Msg("couple: фото загружены, открываю выбранный режим")

	// Режим выбран до фото, поэтому возвращаемся ровно в него. Ноль остаётся
	// для состояний, записанных до этапа 13: у них узла в состоянии нет.
	if nodeID != 0 {
		if err := openCategoryNode(ctx, fc, d, nodeID, 1); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", nodeID).Msg("ошибка открытия режима после загрузки парных фото")
		}
		return
	}
	if err := showCoupleCategoryPage(ctx, fc, d, 1); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка показа режимов после загрузки парных фото")
	}
}

// coupleAwaitingScreen — каким экраном переспрашивать фото. Узел известен
// из состояния: его записал askCouplePhoto.
func coupleAwaitingScreen(ctx context.Context, fc *Context, d *Deps) string {
	const fallback = "couple_intro"
	if d.CatRepo == nil || fc.State == nil || fc.State.CategoryID == 0 {
		return fallback
	}
	cat, err := d.CatRepo.GetByID(ctx, fc.State.CategoryID)
	if err != nil || cat == nil {
		return fallback
	}
	return nodeStepScreen(ctx, d, cat, photoScreenOf, fallback)
}

func HandleCouplePage(ctx context.Context, fc *Context, d *Deps) {
	page := normalizePage(fc.Callback.Page)
	if err := showCoupleCategoryPage(ctx, fc, d, page); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", page).Msg("ошибка отправки страницы couple категорий")
	}
}

func HandleCoupleBrowse(ctx context.Context, fc *Context, d *Deps) {
	switch fc.State.Step {
	case StepCoupleCategories:
		if err := showCoupleCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("ошибка повторной отправки couple категорий")
		}
	case StepCouplePrompts:
		if fc.State.CategoryID == 0 {
			if err := showCoupleCategoryPage(ctx, fc, d, normalizePage(fc.State.CategoryPage)); err != nil {
				log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", fc.State.CategoryPage).Msg("ошибка возврата к couple категориям")
			}
			return
		}
		if err := showPromptPage(ctx, fc, d, fc.State.CategoryID, normalizePage(fc.State.CategoryPage), normalizePage(fc.State.PromptPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", fc.State.CategoryID).Int("page", fc.State.PromptPage).Msg("ошибка повторной отправки couple шаблонов")
		}
	}
}

func showCoupleCategoryPage(ctx context.Context, fc *Context, d *Deps, page int) error {
	return showSectionRoots(ctx, fc, d, specForSection(repository.SectionCouple), page)
}

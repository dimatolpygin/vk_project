package flows

import (
	"context"

	"github.com/rs/zerolog/log"
)

func HandleCoupleStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	// Сначала ждём фото, категории показываем только после загрузки.
	state := copyPrefs(&State{
		Step:       StepCoupleAwaitingPhoto,
		PromptType: "couple",
	}, fc.State)
	state.CouplePhotoURLs = nil
	state.InputPhotoURLs = nil
	state.PhotoBatchID = ""
	_ = d.State.Set(ctx, fc.VkID, state)

	if err := sendScreen(ctx, d, fc.VkID, "couple_intro", ScreenOptions{}); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка отправки интро парных фото")
	}
}

// HandleCoupleAwaitingPhoto принимает фото пары/семьи и только после успешной
// загрузки показывает кнопки с категориями.
func HandleCoupleAwaitingPhoto(ctx context.Context, fc *Context, d *Deps) {
	photos := normalizeGenerationInputPhotos(messagePhotos(fc.Message))
	log.Info().Int64("vk_id", fc.VkID).Int("message_photo_count", len(photos)).Msg("couple: фото во входящем сообщении")

	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "couple_intro", ScreenOptions{})
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
		_ = sendScreen(ctx, d, fc.VkID, "couple_intro", ScreenOptions{})
		return
	}

	carried := copyPrefs(&State{
		Step:       StepCoupleCategories,
		PromptType: "couple",
	}, fc.State)
	carried.CouplePhotoURLs = uploadedURLs
	carried.InputPhotoURLs = nil
	carried.PhotoBatchID = ""
	fc.State = carried

	log.Info().Int64("vk_id", fc.VkID).Int("photo_count", len(uploadedURLs)).Msg("couple: фото загружены, показываю категории")

	if err := showCoupleCategoryPage(ctx, fc, d, 1); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка показа категорий после загрузки парных фото")
	}
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
	categories, err := d.CatRepo.ListActiveCouple(ctx)
	if err != nil || len(categories) == 0 {
		return sendScreen(ctx, d, fc.VkID, "categories_empty", ScreenOptions{})
	}

	rows, currentPage, _ := buildPaginatedRows(categoryButtons(categories), page, "couple_page", nil)
	state := copyPrefs(&State{
		Step:         StepCoupleCategories,
		PromptType:   "couple",
		CategoryPage: currentPage,
		PromptPage:   1,
	}, fc.State)
	_ = d.State.Set(ctx, fc.VkID, state)
	return sendScreen(ctx, d, fc.VkID, "couple_categories", ScreenOptions{PrefixRows: rows})
}

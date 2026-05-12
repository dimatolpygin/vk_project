package flows

import (
	"context"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

func HandleSavedPhotoStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepMainMenu}, fc.State))
	showSavedPhotoMenu(ctx, fc, d)
}

func showSavedPhotoMenu(ctx context.Context, fc *Context, d *Deps) {
	if len(fc.User.SavedPhotoURLs) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_empty", ScreenOptions{})
		return
	}
	atts := nonEmpty(fc.User.SavedPhotoAttachments)
	if len(atts) > 0 {
		log.Info().Int64("vk_id", fc.VkID).Int("att_count", len(atts)).Msg("showing saved photo menu via attachments")
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_filled", ScreenOptions{
			Data:                map[string]any{"Status": savedPhotoStatus(fc.User.UseSavedPhoto)},
			AttachmentsOverride: atts,
			ToggleOn:            fc.User.UseSavedPhoto,
		})
		return
	}
	// Fallback для старых записей без attachment strings
	log.Info().Int64("vk_id", fc.VkID).Msg("showing saved photo menu via URL fallback")
	photoURL := fc.User.SavedPhotoURLs[0]
	_ = sendScreen(ctx, d, fc.VkID, "saved_photo_filled", ScreenOptions{
		Data:          map[string]any{"Status": savedPhotoStatus(fc.User.UseSavedPhoto)},
		ImageOverride: &photoURL,
		ToggleOn:      fc.User.UseSavedPhoto,
	})
}

func HandleSavedPhotoUploadStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingSavedPhoto}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "saved_photo_upload_prompt", ScreenOptions{})
}

func HandleSavedPhotoReceived(ctx context.Context, fc *Context, d *Deps) {
	photoCount := 0
	if fc.Message != nil {
		photoCount = len(fc.Message.Photos)
	}
	log.Info().Int64("vk_id", fc.VkID).Int("photo_count", photoCount).Msg("HandleSavedPhotoReceived")
	if fc.Message == nil || len(fc.Message.Photos) == 0 {
		log.Warn().Int64("vk_id", fc.VkID).Msg("saved photo: no photos in message, re-prompting")
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_upload_prompt", ScreenOptions{})
		return
	}

	urls := fc.Message.Photos
	atts := fc.Message.PhotoAttachments
	if len(atts) != len(urls) {
		atts = make([]string, len(urls))
	}

	if err := d.UserRepo.SetSavedPhotos(ctx, fc.VkID, urls, atts); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить фото пользователя")
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_error", ScreenOptions{})
		return
	}
	log.Info().Int64("vk_id", fc.VkID).Int("count", len(urls)).Msg("saved photos stored")

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventSavedPhotoSaved, "saved_photo_upload", "saved_photo_saved", map[string]any{
		"photo_count": len(urls),
	})
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)

	displayAtts := nonEmpty(atts)
	if len(displayAtts) > 0 {
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_saved", ScreenOptions{
			Data:                map[string]any{"Status": savedPhotoStatus(fc.User.UseSavedPhoto)},
			AttachmentsOverride: displayAtts,
			ToggleOn:            fc.User.UseSavedPhoto,
		})
	} else {
		previewURL := urls[0]
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_saved", ScreenOptions{
			Data:          map[string]any{"Status": savedPhotoStatus(fc.User.UseSavedPhoto)},
			ImageOverride: &previewURL,
			ToggleOn:      fc.User.UseSavedPhoto,
		})
	}
}

func HandleToggleSavedPhoto(ctx context.Context, fc *Context, d *Deps) {
	newValue := !fc.User.UseSavedPhoto
	if err := d.UserRepo.SetUseSavedPhoto(ctx, fc.VkID, newValue); err != nil {
		log.Error().Err(err).Msg("не удалось обновить настройку use_saved_photo")
		return
	}
	trackEvent(ctx, d, fc.VkID, repository.ActivityEventSavedPhotoToggled, "toggle_saved_photo", "saved_photo_filled", map[string]any{
		"enabled": newValue,
	})
	fc.User.UseSavedPhoto = newValue
	showSavedPhotoMenu(ctx, fc, d)
}

func savedPhotoStatus(enabled bool) string {
	if enabled {
		return "✅ вкл"
	}
	return "❌ выкл"
}

// nonEmpty возвращает срез, отфильтрованный от пустых строк.
func nonEmpty(ss []string) []string {
	result := ss[:0:0]
	for _, s := range ss {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

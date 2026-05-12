package flows

import (
	"context"
	"fmt"
	"time"

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
	photos := normalizeGenerationInputPhotos(messagePhotos(fc.Message))
	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_upload_prompt", ScreenOptions{})
		return
	}

	ts := time.Now().Unix()
	finalURLs := make([]string, 0, len(photos))
	for i, photoURL := range photos {
		url := photoURL
		if d.Storage != nil {
			key := fmt.Sprintf("saved_photo/%d/%d_%d.png", fc.VkID, ts, i)
			if _, err := d.Storage.UploadFromURL(ctx, key, photoURL); err != nil {
				log.Error().Err(err).Msg("не удалось загрузить сохранённое фото в S3")
			} else {
				url = d.Storage.PublicURL(key)
			}
		}
		finalURLs = append(finalURLs, url)
	}

	if err := d.UserRepo.SetSavedPhotos(ctx, fc.VkID, finalURLs); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить фото пользователя")
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_error", ScreenOptions{})
		return
	}

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventSavedPhotoSaved, "saved_photo_upload", "saved_photo_saved", map[string]any{
		"has_storage": d.Storage != nil,
		"photo_count": len(finalURLs),
	})
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	previewURL := finalURLs[0]
	_ = sendScreen(ctx, d, fc.VkID, "saved_photo_saved", ScreenOptions{
		Data:          map[string]any{"Status": savedPhotoStatus(fc.User.UseSavedPhoto)},
		ImageOverride: &previewURL,
		ToggleOn:      fc.User.UseSavedPhoto,
	})
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

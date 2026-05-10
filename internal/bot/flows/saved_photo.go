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
	if fc.User.SavedPhotoURL == nil || *fc.User.SavedPhotoURL == "" {
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_empty", ScreenOptions{})
		return
	}

	photoURL := *fc.User.SavedPhotoURL
	_ = sendScreen(ctx, d, fc.VkID, "saved_photo_filled", ScreenOptions{
		Data: map[string]any{
			"Status": savedPhotoStatus(fc.User.UseSavedPhoto),
		},
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
	if len(photos) > 1 {
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_batch_not_supported", ScreenOptions{})
		return
	}

	photoURL := photos[0]
	if d.Storage != nil {
		key := fmt.Sprintf("saved_photo/%d/%d.png", fc.VkID, time.Now().Unix())
		if _, err := d.Storage.UploadFromURL(ctx, key, photoURL); err != nil {
			log.Error().Err(err).Msg("не удалось загрузить сохранённое фото в S3")
		} else {
			photoURL = d.Storage.PublicURL(key)
		}
	}

	if err := d.UserRepo.SetSavedPhoto(ctx, fc.VkID, photoURL); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить фото пользователя")
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_error", ScreenOptions{})
		return
	}

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventSavedPhotoSaved, "saved_photo_upload", "saved_photo_saved", map[string]any{
		"has_storage": d.Storage != nil,
	})
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = sendScreen(ctx, d, fc.VkID, "saved_photo_saved", ScreenOptions{
		Data: map[string]any{
			"Status": savedPhotoStatus(fc.User.UseSavedPhoto),
		},
		ImageOverride: &photoURL,
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

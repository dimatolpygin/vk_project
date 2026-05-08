package flows

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
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
	if fc.Message == nil || len(fc.Message.Photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "saved_photo_upload_prompt", ScreenOptions{})
		return
	}

	photoURL := fc.Message.Photos[0]
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
	fc.User.UseSavedPhoto = newValue
	showSavedPhotoMenu(ctx, fc, d)
}

func savedPhotoStatus(enabled bool) string {
	if enabled {
		return "✅ вкл"
	}
	return "❌ выкл"
}

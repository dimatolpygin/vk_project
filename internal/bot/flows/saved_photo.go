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
		_ = d.Sender.SendText(ctx, fc.VkID,
			"💾 Раздел «Запомнить фото»\n\nЗагрузите базовое фото — оно будет автоматически использоваться в готовых промтах и парных фото:",
			KbSavedPhotoMenu(false, false))
		return
	}
	status := "❌ выкл"
	if fc.User.UseSavedPhoto {
		status = "✅ вкл"
	}
	_ = d.Sender.SendPhoto(ctx, fc.VkID, *fc.User.SavedPhotoURL,
		fmt.Sprintf("💾 Ваше сохранённое фото\n🔄 Использование в готовых промтах: %s", status),
		KbSavedPhotoMenu(true, fc.User.UseSavedPhoto))
}

func HandleSavedPhotoUploadStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingSavedPhoto}, fc.State))
	_ = d.Sender.SendText(ctx, fc.VkID, "📤 Отправьте фото — оно сохранится и будет использоваться по умолчанию:", KbBack())
}

func HandleSavedPhotoReceived(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || len(fc.Message.Photos) == 0 {
		_ = d.Sender.SendText(ctx, fc.VkID, "📤 Отправьте фото — оно сохранится и будет использоваться по умолчанию:", KbBack())
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
		_ = d.Sender.SendText(ctx, fc.VkID, "❌ Ошибка сохранения. Попробуй позже.", KbBack())
		return
	}

	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	status := "❌ выкл"
	if fc.User.UseSavedPhoto {
		status = "✅ вкл"
	}
	_ = d.Sender.SendPhoto(ctx, fc.VkID, photoURL,
		fmt.Sprintf("✅ Фото сохранено!\n🔄 Использование в готовых промтах: %s", status),
		KbSavedPhotoMenu(true, fc.User.UseSavedPhoto))
}

func HandleToggleSavedPhoto(ctx context.Context, fc *Context, d *Deps) {
	newVal := !fc.User.UseSavedPhoto
	if err := d.UserRepo.SetUseSavedPhoto(ctx, fc.VkID, newVal); err != nil {
		log.Error().Err(err).Msg("не удалось обновить настройку use_saved_photo")
		return
	}
	fc.User.UseSavedPhoto = newVal
	showSavedPhotoMenu(ctx, fc, d)
}

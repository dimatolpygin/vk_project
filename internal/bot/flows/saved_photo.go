package flows

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

func HandleSavedPhotoStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingSavedPhoto}, fc.State))
	_ = d.Sender.SendMsg(ctx, fc.VkID, "saved_photo_intro", KbBack())
}

func HandleSavedPhotoReceived(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || len(fc.Message.Photos) == 0 {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "saved_photo_intro", KbBack())
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
	}
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = d.Sender.SendText(ctx, fc.VkID, "✅ Фото сохранено! Оно будет использоваться по умолчанию в готовых промтах и парном фото.", KbMainMenu())
}

func HandleUseSavedPhoto(ctx context.Context, fc *Context, d *Deps) {
	if fc.User.SavedPhotoURL == nil || *fc.User.SavedPhotoURL == "" {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
		return
	}
	fc.Message = &InMessage{Photos: []string{*fc.User.SavedPhotoURL}}
	HandleAwaitingPhoto(ctx, fc, d)
}

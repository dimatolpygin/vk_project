package flows

import (
	"context"

	"github.com/rs/zerolog/log"
)

func HandleSavedPhotoStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhoto, PromptType: "saved_photo_upload"}, fc.State))
	_ = d.Sender.SendMsg(ctx, fc.VkID, "saved_photo_intro", KbBack())
}

func HandleSavedPhotoReceived(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || len(fc.Message.Photos) == 0 {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "saved_photo_intro", KbBack())
		return
	}
	photoURL := fc.Message.Photos[0]
	if err := d.UserRepo.SetSavedPhoto(ctx, fc.VkID, photoURL); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить фото пользователя")
	}
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = d.Sender.SendText(ctx, fc.VkID, "✅ Фото сохранено! Оно будет использоваться по умолчанию.", KbMainMenu())
}

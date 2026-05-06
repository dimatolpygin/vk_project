package flows

import "context"

func HandleEditPhotoStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepAwaitingPhotoEdit, PromptType: "edit"})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "edit_photo_intro", KbBack())
}

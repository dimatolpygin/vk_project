package flows

import "context"

func HandleCoupleStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepAwaitingPhoto, PromptType: "couple"})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "couple_intro", KbBack())
}

package flows

import "context"

func HandleCustomPromptStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
		return
	}
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPrompt, PromptType: "custom"}, fc.State))
	_ = d.Sender.SendMsg(ctx, fc.VkID, "custom_prompt_intro", KbBack())
}

package flows

import "context"

func HandleCustomPromptStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPrompt, PromptType: "custom"}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "custom_prompt_intro", ScreenOptions{})
}

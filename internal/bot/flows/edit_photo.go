package flows

import "context"

func HandleEditPhotoStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhotoEdit, PromptType: "edit"}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
}

func HandleEditResultStart(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{
		Step:     StepAwaitingResultEdit,
		PhotoURL: fc.State.PhotoURL,
	}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
}

func HandleResultEditPrompt(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || fc.Message.Text == "" {
		_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
		return
	}

	photoURL := fc.State.PhotoURL
	if photoURL == "" {
		_ = sendScreen(ctx, d, fc.VkID, "edit_result_missing_photo", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	launchEditGeneration(ctx, fc, d, photoURL, fc.Message.Text)
}

func launchEditGeneration(ctx context.Context, fc *Context, d *Deps, photoURL, prompt string) {
	createAndEnqueueGeneration(ctx, fc, d, "edit", photoURL, prompt, "generating_wait", "edit_result", nil)
}

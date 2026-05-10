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
		Step:           StepAwaitingResultEdit,
		PhotoURL:       fc.State.PhotoURL,
		InputPhotoURLs: nil,
		PhotoBatchID:   "",
	}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
}

func HandleResultEditPrompt(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || fc.Message.Text == "" {
		_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
		return
	}

	photoURLs := generationInputPhotosFromState(fc.State)
	if len(photoURLs) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "edit_result_missing_photo", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	launchEditGeneration(ctx, fc, d, photoURLs, fc.Message.Text)
}

func launchEditGeneration(ctx context.Context, fc *Context, d *Deps, photoURLs []string, prompt string) {
	createAndEnqueueGeneration(ctx, fc, d, "edit", photoURLs, prompt, "generating_wait", "edit_result", nil)
}

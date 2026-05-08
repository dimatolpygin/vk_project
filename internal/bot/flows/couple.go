package flows

import "context"

func HandleCoupleStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	categories, err := d.CatRepo.ListActiveCouple(ctx)
	if err != nil || len(categories) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "categories_empty", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepCoupleMenu, PromptType: "couple"}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "couple_intro", ScreenOptions{PrefixRows: categoryRows(categories)})
}

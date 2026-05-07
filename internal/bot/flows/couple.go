package flows

import "context"

func HandleCoupleStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = d.Sender.SendMsg(ctx, fc.VkID, "no_gens_left", KbBack())
		return
	}
	HandleCoupleMenu(ctx, fc, d)
}

func HandleCoupleMenu(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepCoupleMenu})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "couple_intro", KbCoupleMenu())
}

func HandleCouplePairMenu(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepCoupleMenu, PromptType: "couple_pair"})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "couple_pair_styles", KbCouplePrompts("pair"))
}

func HandleCoupleFamilyMenu(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.Set(ctx, fc.VkID, &State{Step: StepCoupleMenu, PromptType: "couple_family"})
	_ = d.Sender.SendMsg(ctx, fc.VkID, "couple_family_styles", KbCouplePrompts("family"))
}

func HandleCoupleSelectStyle(ctx context.Context, fc *Context, d *Deps, prompt string) {
	state := fc.State
	state.Step = StepAwaitingPhoto
	state.CustomPrompt = prompt
	_ = d.State.Set(ctx, fc.VkID, state)
	_ = d.Sender.SendMsg(ctx, fc.VkID, "photo_requirements", KbBack())
}

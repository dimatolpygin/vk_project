package flows

import "context"

func HandleMainMenu(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = sendScreen(ctx, d, fc.VkID, "bottom_menu", ScreenOptions{})
	_ = sendScreen(ctx, d, fc.VkID, "main_menu", ScreenOptions{})
}

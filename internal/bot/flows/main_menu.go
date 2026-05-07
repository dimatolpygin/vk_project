package flows

import "context"

func HandleMainMenu(ctx context.Context, fc *Context, d *Deps) {
	_ = d.State.SetStep(ctx, fc.VkID, StepMainMenu)
	_ = d.Sender.SendText(ctx, fc.VkID, "─", KbBottomMenu())
	_ = d.Sender.SendMsg(ctx, fc.VkID, "main_menu", KbMainMenu())
}

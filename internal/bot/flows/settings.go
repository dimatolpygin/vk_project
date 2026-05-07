package flows

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

func HandleSettings(ctx context.Context, fc *Context, d *Deps) {
	resolution := fc.State.Resolution
	if resolution == "" {
		resolution = fc.User.PrefResolution
	}
	if resolution == "" {
		resolution = "1k"
	}
	aspectRatio := fc.State.AspectRatio
	if aspectRatio == "" {
		aspectRatio = fc.User.PrefAspectRatio
	}
	if aspectRatio == "" {
		aspectRatio = "авто"
	}
	totalGens := fc.User.FreeGens + fc.User.PaidGens

	model := fc.State.Model
	if model == "" {
		model = fc.User.PrefModel
	}
	if model == "" {
		model = d.DefaultModel
	}

	text := fmt.Sprintf("⚙️ Настройки\n\n"+
		"🎯 Баланс генераций: %d\n"+
		"🤖 Модель: %s\n"+
		"🔧 Качество: %s\n"+
		"📐 Формат: %s",
		totalGens, ModelDisplayName(model), resolution, aspectRatio)

	prevStep := fc.State.Step
	if prevStep == StepSettings {
		prevStep = fc.State.PrevStep
	}
	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:        StepSettings,
		PrevStep:    prevStep,
		PhotoURL:    fc.State.PhotoURL,
		Resolution:  fc.State.Resolution,
		AspectRatio: fc.State.AspectRatio,
		Model:       fc.State.Model,
	})
	_ = d.Sender.SendText(ctx, fc.VkID, text, KbSettings())
}

func ModelDisplayName(modelID string) string {
	switch modelID {
	case "google/nano-banana-pro":
		return "Nano Banana Pro"
	case "google/nano-banana-2":
		return "Nano Banana 2"
	case "openai/gpt-image-2":
		return "GPT Image 2"
	default:
		if modelID == "" {
			return "по умолчанию"
		}
		return modelID
	}
}

func HandleQuality(ctx context.Context, fc *Context, d *Deps) {
	resolution := fc.State.Resolution
	if resolution == "" {
		resolution = "1k"
	}
	_ = d.Sender.SendText(ctx, fc.VkID, "🔧 Выбери качество генерации:", KbQuality(resolution))
}

func HandleFormat(ctx context.Context, fc *Context, d *Deps) {
	_ = d.Sender.SendText(ctx, fc.VkID, "📐 Выбери формат изображения:", KbAspectRatio(fc.State.AspectRatio))
}

func HandleBalance(ctx context.Context, fc *Context, d *Deps) {
	text := fmt.Sprintf("💳 Баланс\n\n🔹 Бесплатных: %d\n🔷 Платных: %d\n\nПополнить баланс:",
		fc.User.FreeGens, fc.User.PaidGens)
	tariffs, _ := d.TariffRepo.ListActive(ctx)
	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:        StepTariffs,
		PrevStep:    StepSettings,
		PhotoURL:    fc.State.PhotoURL,
		Model:       fc.State.Model,
		Resolution:  fc.State.Resolution,
		AspectRatio: fc.State.AspectRatio,
	})
	_ = d.Sender.SendText(ctx, fc.VkID, text, KbTariffs(tariffs))
}

func HandleSetResolution(ctx context.Context, fc *Context, d *Deps, resolution string) {
	fc.State.Resolution = resolution
	fc.User.PrefResolution = resolution
	if err := d.UserRepo.SaveSettings(ctx, fc.VkID, fc.State.Model, resolution, fc.State.AspectRatio); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить настройки пользователя")
	}
	HandleSettings(ctx, fc, d)
}

func HandleSetAspectRatio(ctx context.Context, fc *Context, d *Deps, ar string) {
	fc.State.AspectRatio = ar
	fc.User.PrefAspectRatio = ar
	if err := d.UserRepo.SaveSettings(ctx, fc.VkID, fc.State.Model, fc.State.Resolution, ar); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить настройки пользователя")
	}
	HandleSettings(ctx, fc, d)
}

func HandleModel(ctx context.Context, fc *Context, d *Deps) {
	current := fc.State.Model
	if current == "" {
		current = d.DefaultModel
	}
	_ = d.Sender.SendText(ctx, fc.VkID, "🤖 Выбери модель генерации:", KbModel(current))
}

func HandleSetModel(ctx context.Context, fc *Context, d *Deps, modelID string) {
	fc.State.Model = modelID
	fc.User.PrefModel = modelID
	if err := d.UserRepo.SaveSettings(ctx, fc.VkID, modelID, fc.State.Resolution, fc.State.AspectRatio); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить настройки пользователя")
	}
	HandleSettings(ctx, fc, d)
}

func HandleSupport(ctx context.Context, fc *Context, d *Deps) {
	_ = d.Sender.SendMsg(ctx, fc.VkID, "support_text", KbBack())
}

func HandleExamples(ctx context.Context, fc *Context, d *Deps) {
	_ = d.Sender.SendMsg(ctx, fc.VkID, "examples_collage", KbBack())
}

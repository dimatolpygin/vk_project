package flows

import (
	"context"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

func HandleSettings(ctx context.Context, fc *Context, d *Deps) {
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

	referralCount, referralLink := referralStatus(ctx, fc, d)

	_ = sendScreen(ctx, d, fc.VkID, "settings_overview", ScreenOptions{
		Data: map[string]any{
			"TotalGens":     fc.User.TotalGens(),
			"VKID":          fc.VkID,
			"ModelName":     ModelDisplayName(currentModel(fc, d)),
			"Resolution":    currentResolution(fc),
			"AspectRatio":   currentAspectRatioLabel(fc),
			"ReferralCount": referralCount,
			"ReferralLink":  referralLink,
		},
	})
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
	_ = sendScreen(ctx, d, fc.VkID, "settings_quality", ScreenOptions{
		SelectedValue: currentResolution(fc),
	})
}

func HandleFormat(ctx context.Context, fc *Context, d *Deps) {
	_ = sendScreen(ctx, d, fc.VkID, "settings_format", ScreenOptions{
		SelectedValue: currentAspectRatio(fc),
	})
}

func HandleBalance(ctx context.Context, fc *Context, d *Deps) {
	var tariffs []*repository.Tariff
	if d.TariffRepo != nil {
		tariffs, _ = d.TariffRepo.ListActive(ctx)
	}
	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:        StepTariffs,
		PrevStep:    StepSettings,
		PhotoURL:    fc.State.PhotoURL,
		Model:       fc.State.Model,
		Resolution:  fc.State.Resolution,
		AspectRatio: fc.State.AspectRatio,
	})
	_ = sendScreen(ctx, d, fc.VkID, "settings_balance", ScreenOptions{
		Data: map[string]any{
			"TotalGens": fc.User.TotalGens(),
		},
		PrefixRows: tariffRows(tariffs),
	})
}

func HandleSetResolution(ctx context.Context, fc *Context, d *Deps, resolution string) {
	fc.State.Resolution = resolution
	fc.User.PrefResolution = resolution
	if err := d.UserRepo.SaveSettings(ctx, fc.VkID, fc.State.Model, resolution, fc.State.AspectRatio); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить настройки пользователя")
	} else {
		trackEvent(ctx, d, fc.VkID, repository.ActivityEventSettingsChanged, "quality_"+resolution, "settings_quality", map[string]any{
			"resolution": resolution,
		})
	}
	HandleSettings(ctx, fc, d)
}

func HandleSetAspectRatio(ctx context.Context, fc *Context, d *Deps, ar string) {
	fc.State.AspectRatio = ar
	fc.User.PrefAspectRatio = ar
	if err := d.UserRepo.SaveSettings(ctx, fc.VkID, fc.State.Model, fc.State.Resolution, ar); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить настройки пользователя")
	} else {
		trackEvent(ctx, d, fc.VkID, repository.ActivityEventSettingsChanged, "ar_"+normalizeAspectRatioKey(ar), "settings_format", map[string]any{
			"aspect_ratio": ar,
		})
	}
	HandleSettings(ctx, fc, d)
}

func HandleModel(ctx context.Context, fc *Context, d *Deps) {
	_ = sendScreen(ctx, d, fc.VkID, "settings_model", ScreenOptions{
		SelectedValue: currentModel(fc, d),
	})
}

func HandleSetModel(ctx context.Context, fc *Context, d *Deps, modelID string) {
	fc.State.Model = modelID
	fc.User.PrefModel = modelID
	if err := d.UserRepo.SaveSettings(ctx, fc.VkID, modelID, fc.State.Resolution, fc.State.AspectRatio); err != nil {
		log.Error().Err(err).Msg("не удалось сохранить настройки пользователя")
	} else {
		trackEvent(ctx, d, fc.VkID, repository.ActivityEventSettingsChanged, modelActionKey(modelID), "settings_model", map[string]any{
			"model": modelID,
		})
	}
	HandleSettings(ctx, fc, d)
}

func HandleSupport(ctx context.Context, fc *Context, d *Deps) {
	_ = sendScreen(ctx, d, fc.VkID, "support_text", ScreenOptions{})
}

func HandleExamples(ctx context.Context, fc *Context, d *Deps) {
	_ = sendScreen(ctx, d, fc.VkID, "examples_collage", ScreenOptions{})
}

func modelActionKey(modelID string) string {
	switch modelID {
	case "google/nano-banana-pro":
		return "model_nbp"
	case "google/nano-banana-2":
		return "model_nb2"
	case "openai/gpt-image-2":
		return "model_gpt2"
	default:
		return "model"
	}
}

func normalizeAspectRatioKey(ar string) string {
	switch ar {
	case "1:1":
		return "1_1"
	case "9:16":
		return "9_16"
	case "16:9":
		return "16_9"
	default:
		return ar
	}
}

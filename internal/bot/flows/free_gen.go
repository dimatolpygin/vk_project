package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/content"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/worker"
)

const (
	freeGenerationPromptMessageKey = "free_gen_prompt"
	maxGenerationInputPhotos       = 6
)

func HandleAfterGen(ctx context.Context, fc *Context, d *Deps) {
	if fc.State.PhotoURL != "" {
		_ = showAfterGenScreen(ctx, fc, d, fc.State.PhotoURL)
		return
	}
	if fc.User.HasGens() {
		HandleMainMenu(ctx, fc, d)
	} else {
		HandleWelcome(ctx, fc, d)
	}
}

func HandleFreeGenStart(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		HandleShowTariffs(ctx, fc, d)
		return
	}

	isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
	if err != nil {
		log.Error().Err(err).Msg("РѕС€РёР±РєР° РїСЂРѕРІРµСЂРєРё РїРѕРґРїРёСЃРєРё")
		isMember = false
	}

	if !isMember {
		_ = sendScreen(ctx, d, fc.VkID, "subscribe_cta", ScreenOptions{
			Links: map[string]string{"subscribe_group": d.VKGroupURL},
		})
		_ = d.State.SetStep(ctx, fc.VkID, StepFreeGenStart)
		return
	}

	proceedToPhotoRequest(ctx, fc, d, "free")
}

func HandleCheckSubscription(ctx context.Context, fc *Context, d *Deps) {
	isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
	if err != nil || !isMember {
		_ = sendScreen(ctx, d, fc.VkID, "subscribe_cta", ScreenOptions{
			Links: map[string]string{"subscribe_group": d.VKGroupURL},
		})
		return
	}

	_ = d.UserRepo.SetSubscribed(ctx, fc.VkID, true)
	trackEvent(ctx, d, fc.VkID, repository.ActivityEventSubscriptionConfirmed, "check_sub", "subscribe_cta", nil)
	proceedToPhotoRequest(ctx, fc, d, "free")
}

func proceedToPhotoRequest(ctx context.Context, fc *Context, d *Deps, promptType string) {
	if fc.User.Gender == "unknown" {
		_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingGender, PromptType: promptType}, fc.State))
		_ = sendScreen(ctx, d, fc.VkID, "gender_select", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, copyPrefs(&State{Step: StepAwaitingPhoto, PromptType: promptType}, fc.State))
	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleGenAgain(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		HandleShowTariffs(ctx, fc, d)
		return
	}

	if fc.User.Status != "paid" && fc.User.PaidGens == 0 {
		isMember, err := d.VKClient.IsMember(ctx, fc.VkID)
		if err != nil {
			isMember = false
		}
		if !isMember {
			_ = sendScreen(ctx, d, fc.VkID, "subscribe_cta", ScreenOptions{
				Links: map[string]string{"subscribe_group": d.VKGroupURL},
			})
			_ = d.State.SetStep(ctx, fc.VkID, StepFreeGenStart)
			return
		}
	}

	if fc.User.Gender == "unknown" {
		savedState := *fc.State
		savedState.Step = StepAwaitingGender
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = sendScreen(ctx, d, fc.VkID, "gender_select", ScreenOptions{})
		return
	}

	savedState := *fc.State
	savedState.PhotoURL = ""
	savedState.InputPhotoURLs = nil

	if fc.State.PromptType == "edit" {
		savedState.Step = StepAwaitingPhotoEdit
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
	} else {
		savedState.Step = StepAwaitingPhoto
		_ = d.State.Set(ctx, fc.VkID, &savedState)
		_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
	}
}

func HandleGenderSelect(ctx context.Context, fc *Context, d *Deps, gender string) {
	if d.UserRepo != nil {
		_ = d.UserRepo.SetGender(ctx, fc.VkID, gender)
	}
	fc.User.Gender = gender

	promptType := fc.State.PromptType
	if promptType == "" {
		promptType = "free"
	}

	if promptType == "ready_prompt" {
		if err := showReadyPromptsCategoryPage(ctx, fc, d, 1); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("РЅРµ СѓРґР°Р»РѕСЃСЊ РїРѕРєР°Р·Р°С‚СЊ РєР°С‚РµРіРѕСЂРёРё РіРѕС‚РѕРІС‹С… РїСЂРѕРјС‚РѕРІ РїРѕСЃР»Рµ РІС‹Р±РѕСЂР° РїРѕР»Р°")
		}
		return
	}

	newState := *fc.State
	newState.PromptType = promptType
	if promptType == "edit" {
		newState.Step = StepAwaitingPhotoEdit
		_ = d.State.Set(ctx, fc.VkID, &newState)
		_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
		return
	}

	newState.Step = StepAwaitingPhoto
	_ = d.State.Set(ctx, fc.VkID, &newState)
	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleAwaitingPhoto(ctx context.Context, fc *Context, d *Deps) {
	photos := normalizeGenerationInputPhotos(messagePhotos(fc.Message))

	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	uploadedURLs := uploadGenerationInputPhotos(ctx, d, fc.VkID, photos, "user_upload")

	promptType := fc.State.PromptType
	prompt := buildDefaultPrompt(ctx, d, fc.User.Gender, promptType)
	if fc.State.CustomPrompt != "" {
		prompt = fc.State.CustomPrompt
	}
	if fc.State.TemplateID > 0 {
		if templatePrompt, err := d.PromptRepo.GetByID(ctx, fc.State.TemplateID); err == nil && templatePrompt != nil {
			prompt = templatePrompt.Prompt
		}
	}

	startGeneration(ctx, fc, d, uploadedURLs, prompt, promptType, "generating_wait", nil)
}

func startGeneration(ctx context.Context, fc *Context, d *Deps, photoURLs []string, prompt, promptType, waitKey string, waitData map[string]any) {
	createAndEnqueueGeneration(ctx, fc, d, promptType, photoURLs, prompt, waitKey, "free_gen", waitData)
}

func createAndEnqueueGeneration(ctx context.Context, fc *Context, d *Deps, generationType string, photoURLs []string, prompt, waitKey, activityActionKey string, waitData map[string]any) {
	model := fc.State.Model
	if model == "" {
		model = fc.User.PrefModel
	}
	if model == "" {
		model = d.DefaultModel
	}

	inputPhotoURL := firstGenerationInputPhotoURL(photoURLs)
	gen, err := d.GenRepo.CreateChargedGeneration(ctx, fc.VkID, generationType, prompt, model, inputPhotoURL)
	switch {
	case errors.Is(err, repository.ErrNoGenerationsAvailable):
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	case err != nil:
		log.Error().Err(err).Msg("РЅРµ СѓРґР°Р»РѕСЃСЊ СЃРѕР·РґР°С‚СЊ Р·Р°РїРёСЃСЊ РіРµРЅРµСЂР°С†РёРё")
		_ = sendScreen(ctx, d, fc.VkID, "generation_error", ScreenOptions{})
		return
	}

	trackEvent(ctx, d, fc.VkID, repository.ActivityEventGenerationStarted, activityActionKey, waitKey, map[string]any{
		"generation_id": gen.ID,
		"prompt_type":   generationType,
		"model":         model,
		"resolution":    currentResolution(fc),
		"aspect_ratio":  currentAspectRatio(fc),
		"input_photos":  len(photoURLs),
	})

	nextState := *fc.State
	nextState.Step = StepMainMenu
	nextState.InputPhotoURLs = nil
	_ = d.State.Set(ctx, fc.VkID, &nextState)
	_ = sendScreen(ctx, d, fc.VkID, waitKey, ScreenOptions{Data: waitData})

	resolution := currentResolution(fc)
	aspectRatio := currentAspectRatio(fc)
	payload := worker.GeneratePayload{
		GenerationID: gen.ID,
		UserVKID:     fc.VkID,
		Model:        model,
		Images:       clonePhotoURLs(photoURLs),
		Prompt:       prompt,
		Resolution:   resolution,
		AspectRatio:  aspectRatio,
		OutputFormat: "png",
	}
	payloadBytes, _ := payload.Bytes()
	task := asynq.NewTask(worker.TaskGenerate, payloadBytes,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
	)
	if _, err := d.AsynqClient.Enqueue(task); err != nil {
		log.Error().Err(err).Msg("РЅРµ СѓРґР°Р»РѕСЃСЊ РїРѕСЃС‚Р°РІРёС‚СЊ Р·Р°РґР°С‡Сѓ РІ РѕС‡РµСЂРµРґСЊ")
		_ = d.GenRepo.SetFailed(ctx, gen.ID, err.Error())
		if refundErr := d.GenRepo.RefundGenerationCharge(ctx, gen.ID); refundErr != nil {
			log.Error().Err(refundErr).Int64("generation_id", gen.ID).Msg("РЅРµ СѓРґР°Р»РѕСЃСЊ РІРµСЂРЅСѓС‚СЊ РіРµРЅРµСЂР°С†РёСЋ РїРѕСЃР»Рµ enqueue error")
		}
		_ = sendScreen(ctx, d, fc.VkID, "generation_start_error", ScreenOptions{})
	}
}

func HandleAwaitingPrompt(ctx context.Context, fc *Context, d *Deps) {
	if fc.Message == nil || fc.Message.Text == "" {
		_ = sendScreen(ctx, d, fc.VkID, "custom_prompt_intro", ScreenOptions{})
		return
	}

	state := fc.State
	state.Step = StepAwaitingPhoto
	state.CustomPrompt = fc.Message.Text
	_ = d.State.Set(ctx, fc.VkID, state)
	_ = sendScreen(ctx, d, fc.VkID, "photo_requirements", ScreenOptions{})
}

func HandleAwaitingPhotoEdit(ctx context.Context, fc *Context, d *Deps) {
	photos := normalizeGenerationInputPhotos(messagePhotos(fc.Message))
	if len(photos) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "edit_photo_intro", ScreenOptions{})
		return
	}
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}

	uploadedURLs := uploadGenerationInputPhotos(ctx, d, fc.VkID, photos, "edit_upload")

	state := fc.State
	state.Step = StepAwaitingEditPrompt
	state.PhotoURL = ""
	state.InputPhotoURLs = uploadedURLs
	_ = d.State.Set(ctx, fc.VkID, state)

	if state.CustomPrompt != "" {
		fc.State = state
		launchEditGeneration(ctx, fc, d, uploadedURLs, state.CustomPrompt)
		return
	}

	_ = sendScreen(ctx, d, fc.VkID, "edit_result_prompt", ScreenOptions{})
}

func showAfterGenScreen(ctx context.Context, fc *Context, d *Deps, photoURL string) error {
	screenKey := "after_gen_free"
	options := ScreenOptions{ImageOverride: &photoURL}

	if fc.User.PaidGens > 0 || fc.User.Status == "paid" {
		screenKey = "after_gen_paid"
		options.Links = map[string]string{"download_photo": photoURL}
		options.Data = map[string]any{
			"ModelName":   ModelDisplayName(currentModel(fc, d)),
			"Resolution":  currentResolution(fc),
			"AspectRatio": currentAspectRatioLabel(fc),
		}
	}

	return sendScreen(ctx, d, fc.VkID, screenKey, options)
}

func currentModel(fc *Context, d *Deps) string {
	if fc.State.Model != "" {
		return fc.State.Model
	}
	if fc.User.PrefModel != "" {
		return fc.User.PrefModel
	}
	return d.DefaultModel
}

func currentResolution(fc *Context) string {
	if fc.State.Resolution != "" {
		return fc.State.Resolution
	}
	if fc.User.PrefResolution != "" {
		return fc.User.PrefResolution
	}
	return "1k"
}

func currentAspectRatio(fc *Context) string {
	if fc.State.AspectRatio != "" {
		return fc.State.AspectRatio
	}
	return fc.User.PrefAspectRatio
}

func currentAspectRatioLabel(fc *Context) string {
	ar := currentAspectRatio(fc)
	if ar == "" {
		return "Р°РІС‚Рѕ"
	}
	return ar
}

func buildDefaultPrompt(ctx context.Context, d *Deps, gender, promptType string) string {
	switch promptType {
	case "couple_pair":
		return "romantic couple portrait, two people, professional photo, studio lighting, high quality"
	case "couple_family":
		return "family portrait, warm atmosphere, professional photo, studio lighting, high quality"
	case "couple":
		return "couple portrait, two people, professional photo, studio lighting, high quality"
	}

	return renderFreeGenerationPrompt(ctx, d, gender)
}

func renderFreeGenerationPrompt(ctx context.Context, d *Deps, gender string) string {
	rawPrompt := defaultFreeGenerationPromptTemplate(gender)
	if d != nil && d.MsgRepo != nil {
		msg, err := d.MsgRepo.Get(ctx, freeGenerationPromptMessageKey)
		if err != nil {
			log.Error().Err(err).Str("message_key", freeGenerationPromptMessageKey).Msg("failed to load free generation prompt from messages")
		} else if msg != nil && strings.TrimSpace(msg.Text) != "" {
			rawPrompt = msg.Text
		}
	}

	rendered, err := content.RenderText(rawPrompt, map[string]any{
		"Gender":      gender,
		"GenderLabel": defaultPromptGenderLabel(gender),
	})
	if err != nil {
		log.Error().Err(err).Str("message_key", freeGenerationPromptMessageKey).Msg("failed to render free generation prompt template")
		return defaultFreeGenerationPromptTemplate(gender)
	}

	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return defaultFreeGenerationPromptTemplate(gender)
	}
	return rendered
}

func defaultFreeGenerationPromptTemplate(gender string) string {
	return fmt.Sprintf("professional portrait photo of a %s, studio lighting, high quality, photorealistic", defaultPromptGenderLabel(gender))
}

func defaultPromptGenderLabel(gender string) string {
	if gender == "male" {
		return "man"
	}
	return "woman"
}

func messagePhotos(msg *InMessage) []string {
	if msg == nil {
		return nil
	}
	return msg.Photos
}

func normalizeGenerationInputPhotos(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}

	out := make([]string, 0, minInt(len(urls), maxGenerationInputPhotos))
	seen := make(map[string]struct{}, len(urls))
	for _, url := range urls {
		trimmed := strings.TrimSpace(url)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
		if len(out) == maxGenerationInputPhotos {
			break
		}
	}
	return out
}

func uploadGenerationInputPhotos(ctx context.Context, d *Deps, vkID int64, photoURLs []string, storagePrefix string) []string {
	normalized := normalizeGenerationInputPhotos(photoURLs)
	if len(normalized) == 0 {
		return nil
	}
	if d == nil || d.Storage == nil {
		return normalized
	}

	batchID := time.Now().UnixNano()
	uploaded := make([]string, 0, len(normalized))
	for idx, photoURL := range normalized {
		storedURL := photoURL
		key := fmt.Sprintf("%s/%d/%d_%d.png", storagePrefix, vkID, batchID, idx+1)
		if _, err := d.Storage.UploadFromURL(ctx, key, photoURL); err != nil {
			log.Error().Err(err).Str("storage_prefix", storagePrefix).Int("photo_index", idx).Msg("failed to upload input photo to storage, using original url")
		} else {
			storedURL = d.Storage.PublicURL(key)
		}
		uploaded = append(uploaded, storedURL)
	}
	return uploaded
}

func generationInputPhotosFromState(st *State) []string {
	if st == nil {
		return nil
	}
	if len(st.InputPhotoURLs) > 0 {
		return normalizeGenerationInputPhotos(st.InputPhotoURLs)
	}
	if trimmed := strings.TrimSpace(st.PhotoURL); trimmed != "" {
		return []string{trimmed}
	}
	return nil
}

func firstGenerationInputPhotoURL(photoURLs []string) *string {
	if len(photoURLs) == 0 {
		return nil
	}
	first := strings.TrimSpace(photoURLs[0])
	if first == "" {
		return nil
	}
	return &first
}

func clonePhotoURLs(photoURLs []string) []string {
	if len(photoURLs) == 0 {
		return nil
	}
	cloned := make([]string, len(photoURLs))
	copy(cloned, photoURLs)
	return cloned
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

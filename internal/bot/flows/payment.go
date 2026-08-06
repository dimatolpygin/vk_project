package flows

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/worker"
	"vk_neuro_bot/internal/yukassa"
)

const referralBonusGens = 2

type paymentSettler interface {
	SettleSuccessfulPayment(ctx context.Context, paymentID string, paidGensHint int) (*repository.PaymentSettlementResult, error)
}

type paymentCanceler interface {
	CancelPayment(ctx context.Context, paymentID string) (*repository.PaymentCancellationResult, error)
}

func HandleShowTariffs(ctx context.Context, fc *Context, d *Deps) {
	if d.TariffRepo == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	tariffs, err := d.TariffRepo.ListActive(ctx)
	if err != nil || len(tariffs) == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	_ = d.State.Set(ctx, fc.VkID, &State{
		Step:           StepTariffs,
		PrevStep:       fc.State.Step,
		PhotoURL:       fc.State.PhotoURL,
		InputPhotoURLs: clonePhotoURLs(fc.State.InputPhotoURLs),
	})
	_ = sendScreen(ctx, d, fc.VkID, "tariffs", ScreenOptions{PrefixRows: tariffRows(tariffs)})

	schedulePaymentReminder(ctx, d, fc)
}

// schedulePaymentReminder ставит догоняющее сообщение через 3 минуты после показа
// тарифов. Оплатившим оно не уйдёт — воркер перепроверяет баланс перед отправкой.
func schedulePaymentReminder(ctx context.Context, d *Deps, fc *Context) {
	if d.AsynqClient == nil {
		return
	}
	if fc.User != nil && (fc.User.Status == "paid" || fc.User.PaidGens > 0) {
		return
	}

	payloadBytes, err := worker.PaymentReminderPayload{UserVKID: fc.VkID}.Bytes()
	if err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("не удалось собрать payload напоминания об оплате")
		return
	}

	task := asynq.NewTask(
		worker.TaskPaymentReminder,
		payloadBytes,
		asynq.ProcessIn(worker.PaymentReminderDelay),
		asynq.MaxRetry(3),
		asynq.Timeout(time.Minute),
		// Повторные заходы в тарифы внутри окна ожидания не плодят дубли.
		asynq.Unique(worker.PaymentReminderDelay+time.Minute),
	)
	if _, err := d.AsynqClient.Enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			log.Debug().Int64("vk_id", fc.VkID).Msg("напоминание об оплате уже запланировано")
			return
		}
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("не удалось запланировать напоминание об оплате")
		return
	}

	log.Info().
		Int64("vk_id", fc.VkID).
		Dur("delay", worker.PaymentReminderDelay).
		Msg("напоминание об оплате запланировано")
}

// HandleBroadcastCTA обрабатывает кнопку «Сделать такие фото» из рассылки:
// пользователю без генераций сначала показываем нулевой баланс, затем — тарифы.
func HandleBroadcastCTA(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "broadcast_no_gens", ScreenOptions{})
	}
	HandleShowTariffs(ctx, fc, d)
}

func HandleBuyTariff(ctx context.Context, fc *Context, d *Deps) {
	if d.TariffRepo == nil || d.OrderRepo == nil || d.Yukassa == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_unavailable", ScreenOptions{})
		return
	}

	tariffID := fc.Callback.TariffID
	if tariffID == 0 {
		_ = sendScreen(ctx, d, fc.VkID, "payment_invalid_tariff", ScreenOptions{})
		return
	}

	tariff, err := d.TariffRepo.GetByID(ctx, tariffID)
	if err != nil || tariff == nil {
		_ = sendScreen(ctx, d, fc.VkID, "payment_tariff_not_found", ScreenOptions{})
		return
	}

	returnURL := buildPaymentReturnURL(d.VKGroupID)
	if returnURL == "" {
		log.Error().
			Int64("vk_id", fc.VkID).
			Int64("vk_group_id", d.VKGroupID).
			Msg("vk group id is not configured for payment return url")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	order, err := d.OrderRepo.Create(ctx, fc.VkID, tariffID, tariff.Price)
	if err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("tariff_id", tariffID).Msg("failed to create payment order")
		_ = sendScreen(ctx, d, fc.VkID, "payment_order_error", ScreenOptions{})
		return
	}

	var (
		username  string
		firstName string
	)
	if d.UserRepo != nil {
		user, userErr := d.UserRepo.GetByVKID(ctx, fc.VkID)
		if userErr != nil {
			log.Warn().Err(userErr).Int64("vk_id", fc.VkID).Msg("failed to load user before creating yookassa payment")
		} else if user != nil {
			username = strings.TrimSpace(user.Username)
			firstName = strings.TrimSpace(user.FirstName)
		}
	}

	paymentDescription := fmt.Sprintf("\u0417\u0430\u043a\u0430\u0437 \u2116%d", order.ID)
	receiptDescription := strings.TrimSpace(tariff.Description)
	if receiptDescription == "" {
		receiptDescription = strings.TrimSpace(tariff.Name)
	}

	log.Info().
		Int64("order_id", order.ID).
		Int64("vk_id", fc.VkID).
		Int("tariff_id", tariffID).
		Str("return_url", returnURL).
		Msg("creating payment link")

	payment, err := d.Yukassa.CreatePayment(ctx, yukassa.PaymentRequest{
		Amount:                 tariff.Price,
		TariffID:               tariffID,
		GensCount:              tariff.GensCount,
		UserVKID:               fc.VkID,
		OrderID:                order.ID,
		ReturnURL:              returnURL,
		Description:            paymentDescription,
		ReceiptItemDescription: receiptDescription,
		Username:               username,
		FirstName:              firstName,
	})
	if err != nil {
		log.Error().
			Err(err).
			Int64("order_id", order.ID).
			Int64("vk_id", fc.VkID).
			Int("tariff_id", tariffID).
			Msg("failed to create yookassa payment")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	payToken, tokenErr := newPayToken()
	if tokenErr != nil {
		// \u0411\u0435\u0437 \u0442\u043e\u043a\u0435\u043d\u0430 \u0440\u0435\u0434\u0438\u0440\u0435\u043a\u0442-\u0441\u0442\u0440\u0430\u043d\u0438\u0446\u0443 \u043d\u0435 \u0441\u043e\u0431\u0440\u0430\u0442\u044c, \u043d\u043e \u0438 \u0440\u043e\u043d\u044f\u0442\u044c \u043e\u043f\u043b\u0430\u0442\u0443 \u0438\u0437-\u0437\u0430 \u044d\u0442\u043e\u0433\u043e
		// \u043d\u0435\u043b\u044c\u0437\u044f \u2014 \u043e\u0442\u0434\u0430\u0451\u043c \u043a\u043d\u043e\u043f\u043a\u0443 \u0441\u043e \u0441\u0441\u044b\u043b\u043a\u043e\u0439 \u043d\u0430 \u042eKassa \u043d\u0430\u043f\u0440\u044f\u043c\u0443\u044e, \u043a\u0430\u043a \u0431\u044b\u043b\u043e \u0440\u0430\u043d\u044c\u0448\u0435.
		log.Error().Err(tokenErr).Int64("order_id", order.ID).Msg("failed to generate pay token")
	}

	if payToken != "" {
		err = d.OrderRepo.SetPaymentLink(ctx, order.ID, payment.PaymentID, payment.PaymentURL, payToken)
	} else {
		err = d.OrderRepo.SetPaymentID(ctx, order.ID, payment.PaymentID)
	}
	if err != nil {
		log.Error().
			Err(err).
			Int64("order_id", order.ID).
			Str("payment_id", payment.PaymentID).
			Msg("failed to persist yookassa payment id")
		_ = sendScreen(ctx, d, fc.VkID, "payment_link_error", ScreenOptions{})
		return
	}

	buttonURL := buildPayRedirectURL(d.PublicBaseURL, payToken)
	if buttonURL == "" {
		buttonURL = payment.PaymentURL
	}

	log.Info().
		Int64("order_id", order.ID).
		Str("payment_id", payment.PaymentID).
		Str("payment_url", payment.PaymentURL).
		Str("button_url", buttonURL).
		Str("return_url", returnURL).
		Msg("payment link created")

	_ = sendScreen(ctx, d, fc.VkID, "payment_checkout", ScreenOptions{
		Data: map[string]any{
			"TariffName":  tariff.Name,
			"Description": tariff.Description,
			"Price":       fmt.Sprintf("%.0f\u20bd", tariff.Price),
		},
		Links: map[string]string{"payment_url": buttonURL},
	})
}

// newPayToken \u0432\u044b\u0434\u0430\u0451\u0442 \u043f\u0443\u0431\u043b\u0438\u0447\u043d\u044b\u0439 \u0438\u0434\u0435\u043d\u0442\u0438\u0444\u0438\u043a\u0430\u0442\u043e\u0440 \u043f\u043b\u0430\u0442\u0451\u0436\u043d\u043e\u0439 \u0441\u0441\u044b\u043b\u043a\u0438. \u0421\u043b\u0443\u0447\u0430\u0439\u043d\u044b\u0439, \u0430 \u043d\u0435
// id \u0437\u0430\u043a\u0430\u0437\u0430, \u0447\u0442\u043e\u0431\u044b \u043f\u043e \u0441\u0441\u044b\u043b\u043a\u0435 \u043d\u0435\u043b\u044c\u0437\u044f \u0431\u044b\u043b\u043e \u043f\u0435\u0440\u0435\u0431\u0440\u0430\u0442\u044c \u0447\u0443\u0436\u0438\u0435 \u043f\u043b\u0430\u0442\u0435\u0436\u0438.
func newPayToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// buildPayRedirectURL \u0441\u043e\u0431\u0438\u0440\u0430\u0435\u0442 \u0441\u0441\u044b\u043b\u043a\u0443 \u043d\u0430 \u043d\u0430\u0448\u0443 \u0440\u0435\u0434\u0438\u0440\u0435\u043a\u0442-\u0441\u0442\u0440\u0430\u043d\u0438\u0446\u0443. \u041f\u0443\u0441\u0442\u0430\u044f \u0441\u0442\u0440\u043e\u043a\u0430
// \u043e\u0437\u043d\u0430\u0447\u0430\u0435\u0442 \u00ab\u043e\u0442\u0434\u0430\u0442\u044c \u0441\u0441\u044b\u043b\u043a\u0443 \u042eKassa \u043a\u0430\u043a \u0435\u0441\u0442\u044c\u00bb.
func buildPayRedirectURL(publicBaseURL, payToken string) string {
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL == "" || payToken == "" {
		return ""
	}
	return publicBaseURL + "/pay/" + payToken
}

func ProcessSuccessfulPayment(ctx context.Context, d *Deps, paymentID string) error {
	return ProcessSuccessfulPaymentWithMetadata(ctx, d, paymentID, nil)
}

func ProcessSuccessfulPaymentWithMetadata(ctx context.Context, d *Deps, paymentID string, paymentMetadata map[string]any) error {
	return processSuccessfulPayment(ctx, d, d.OrderRepo, paymentID, paymentMetadata)
}

func processSuccessfulPayment(ctx context.Context, d *Deps, settler paymentSettler, paymentID string, paymentMetadata map[string]any) error {
	paidGensHint := gensCountFromPaymentMetadata(paymentMetadata)

	log.Info().
		Str("payment_id", paymentID).
		Int("paid_gens_hint", paidGensHint).
		Msg("processing successful payment")

	result, err := settler.SettleSuccessfulPayment(ctx, paymentID, paidGensHint)
	if err != nil {
		log.Error().Err(err).Str("payment_id", paymentID).Msg("settle successful payment failed")
		return err
	}
	if result == nil {
		log.Warn().Str("payment_id", paymentID).Msg("settle returned nil result")
		return nil
	}
	if result.AlreadyProcessed {
		log.Info().
			Str("payment_id", paymentID).
			Int64("vk_id", result.UserVKID).
			Msg("payment already processed, skipping")
		return nil
	}

	log.Info().
		Str("payment_id", paymentID).
		Int64("vk_id", result.UserVKID).
		Int("gens_added", result.PaidGensAdded).
		Bool("bonus_granted", result.BonusGranted).
		Msg("payment settled, sending notification")

	trackEvent(ctx, d, result.UserVKID, repository.ActivityEventPaymentSucceeded, "buy_tariff", "payment_success", map[string]any{
		"payment_id": paymentID,
		"tariff_id":  result.TariffID,
		"gens_count": result.PaidGensAdded,
	})

	if result.BonusGranted {
		trackEvent(ctx, d, result.ReferrerVKID, repository.ActivityEventReferralBonusAwarded, "referral", "referral_bonus_awarded", map[string]any{
			"referred_vk_id": result.UserVKID,
			"bonus_gens":     referralBonusGens,
		})
		_ = sendScreen(ctx, d, result.ReferrerVKID, "referral_bonus_awarded", ScreenOptions{})
	}

	screenGensCount := result.PaidGensAdded
	if screenGensCount <= 0 {
		screenGensCount = paidGensHint
	}

	_ = sendScreen(ctx, d, result.UserVKID, "payment_success", ScreenOptions{
		Data: map[string]any{
			"GensCount": screenGensCount,
		},
	})
	return nil
}

func ProcessCanceledPayment(ctx context.Context, d *Deps, paymentID string) error {
	return processCanceledPayment(ctx, d, d.OrderRepo, paymentID)
}

func processCanceledPayment(ctx context.Context, d *Deps, canceler paymentCanceler, paymentID string) error {
	result, err := canceler.CancelPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if result == nil || result.AlreadyProcessed {
		return nil
	}

	_ = sendScreen(ctx, d, result.UserVKID, "payment_canceled", ScreenOptions{})
	return nil
}

func buildPaymentReturnURL(groupID int64) string {
	if groupID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://vk.com/write-%d", groupID)
}

func gensCountFromPaymentMetadata(metadata map[string]any) int {
	if len(metadata) == 0 {
		return 0
	}

	rawValue, ok := metadata["gens_count"]
	if !ok {
		return 0
	}

	switch value := rawValue.(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && parsed > 0 {
			return parsed
		}
	}

	return 0
}

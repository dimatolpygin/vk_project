package worker

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

const (
	// PaymentReminderDelay — через сколько после показа тарифов уходит догоняющее сообщение.
	PaymentReminderDelay = 3 * time.Minute
	// paymentReminderCooldown — не беспокоим одного и того же пользователя чаще раза в сутки.
	paymentReminderCooldown = 24 * time.Hour
	// reminderTariffGensCount — тариф по умолчанию для кнопки в напоминании (3 генерации).
	reminderTariffGensCount = 3
	// reminderTariffConfigKey — переопределение тарифа из админ-конфига.
	reminderTariffConfigKey = "reminder_tariff_id"
)

type ReminderUserStore interface {
	GetByVKID(ctx context.Context, vkID int64) (*repository.User, error)
}

type ReminderTariffStore interface {
	ListActive(ctx context.Context) ([]*repository.Tariff, error)
	GetByID(ctx context.Context, id int) (*repository.Tariff, error)
}

type ReminderSender interface {
	SendPaymentReminder(ctx context.Context, vkID int64, tariff *repository.Tariff) error
}

type ReminderActivityStore interface {
	Record(ctx context.Context, event repository.ActivityEvent) error
	LastEventAt(ctx context.Context, vkID int64, eventType string) (*time.Time, error)
}

type ReminderConfigStore interface {
	Get(ctx context.Context, key string) (string, error)
}

type PaymentReminderHandler struct {
	users    ReminderUserStore
	tariffs  ReminderTariffStore
	sender   ReminderSender
	activity ReminderActivityStore
	config   ReminderConfigStore
	cooldown time.Duration
}

func NewPaymentReminderHandler(
	users ReminderUserStore,
	tariffs ReminderTariffStore,
	sender ReminderSender,
	activity ReminderActivityStore,
	config ReminderConfigStore,
) *PaymentReminderHandler {
	return &PaymentReminderHandler{
		users:    users,
		tariffs:  tariffs,
		sender:   sender,
		activity: activity,
		config:   config,
		cooldown: paymentReminderCooldown,
	}
}

func (h *PaymentReminderHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParsePaymentReminderPayload(task.Payload())
	if err != nil {
		return fmt.Errorf("напоминание об оплате: ошибка парсинга payload: %w", err)
	}
	if payload.UserVKID <= 0 {
		return fmt.Errorf("%w: invalid user_vk_id", asynq.SkipRetry)
	}

	user, err := h.users.GetByVKID(ctx, payload.UserVKID)
	if err != nil {
		return err
	}
	if user == nil {
		log.Debug().Int64("vk_id", payload.UserVKID).Msg("напоминание об оплате: пользователь не найден, пропускаем")
		return nil
	}

	// Оплатил за время ожидания — догонять больше нечего.
	if user.Status == "paid" || user.PaidGens > 0 {
		log.Info().Int64("vk_id", user.VKID).Msg("напоминание об оплате: пользователь уже оплатил, пропускаем")
		return nil
	}

	if skip, err := h.recentlyReminded(ctx, user.VKID); err != nil {
		return err
	} else if skip {
		log.Info().Int64("vk_id", user.VKID).Msg("напоминание об оплате: уже отправляли за последние сутки, пропускаем")
		return nil
	}

	tariff, err := h.resolveTariff(ctx)
	if err != nil {
		return err
	}
	if tariff == nil {
		log.Warn().Int64("vk_id", user.VKID).Msg("напоминание об оплате: нет активных тарифов, пропускаем")
		return nil
	}

	if err := h.sender.SendPaymentReminder(ctx, user.VKID, tariff); err != nil {
		return err
	}

	log.Info().
		Int64("vk_id", user.VKID).
		Int("tariff_id", tariff.ID).
		Float64("price", tariff.Price).
		Msg("напоминание об оплате отправлено")

	if h.activity != nil {
		if err := h.activity.Record(ctx, repository.ActivityEvent{
			UserVKID:  user.VKID,
			EventType: repository.ActivityEventPaymentReminderSent,
			ScreenKey: "payment_reminder",
			Meta: map[string]any{
				"tariff_id":  tariff.ID,
				"gens_count": tariff.GensCount,
			},
		}); err != nil {
			log.Warn().Err(err).Int64("vk_id", user.VKID).Msg("не удалось записать событие напоминания об оплате")
		}
	}

	return nil
}

func (h *PaymentReminderHandler) recentlyReminded(ctx context.Context, vkID int64) (bool, error) {
	if h.activity == nil || h.cooldown <= 0 {
		return false, nil
	}
	lastAt, err := h.activity.LastEventAt(ctx, vkID, repository.ActivityEventPaymentReminderSent)
	if err != nil {
		return false, err
	}
	if lastAt == nil {
		return false, nil
	}
	return time.Since(*lastAt) < h.cooldown, nil
}

// resolveTariff выбирает тариф для кнопки: сначала переопределение из админ-конфига,
// затем активный тариф на 3 генерации, иначе — самый дешёвый активный.
func (h *PaymentReminderHandler) resolveTariff(ctx context.Context) (*repository.Tariff, error) {
	if h.config != nil {
		raw, err := h.config.Get(ctx, reminderTariffConfigKey)
		if err != nil {
			log.Warn().Err(err).Msg("не удалось прочитать reminder_tariff_id из админ-конфига")
		} else if id, convErr := strconv.Atoi(strings.TrimSpace(raw)); convErr == nil && id > 0 {
			tariff, err := h.tariffs.GetByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if tariff != nil && tariff.IsActive {
				return tariff, nil
			}
			log.Warn().Int("tariff_id", id).Msg("reminder_tariff_id указывает на отсутствующий или отключённый тариф")
		}
	}

	tariffs, err := h.tariffs.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	if len(tariffs) == 0 {
		return nil, nil
	}

	for _, tariff := range tariffs {
		if tariff != nil && tariff.GensCount == reminderTariffGensCount {
			return tariff, nil
		}
	}

	cheapest := make([]*repository.Tariff, 0, len(tariffs))
	for _, tariff := range tariffs {
		if tariff != nil {
			cheapest = append(cheapest, tariff)
		}
	}
	if len(cheapest) == 0 {
		return nil, nil
	}
	sort.Slice(cheapest, func(i, j int) bool { return cheapest[i].Price < cheapest[j].Price })
	return cheapest[0], nil
}

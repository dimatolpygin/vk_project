package flows

import (
	"context"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

func trackEvent(ctx context.Context, d *Deps, vkID int64, eventType, actionKey, screenKey string, meta map[string]any) {
	if d == nil || d.ActivityRepo == nil || vkID <= 0 || eventType == "" {
		return
	}

	if err := d.ActivityRepo.Record(ctx, repository.ActivityEvent{
		UserVKID:  vkID,
		EventType: eventType,
		ActionKey: actionKey,
		ScreenKey: screenKey,
		Meta:      meta,
	}); err != nil {
		log.Debug().Err(err).Int64("vk_id", vkID).Str("event_type", eventType).Msg("failed to record activity event")
	}
}

package bot

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/repository"
)

type userStore interface {
	GetByVKID(ctx context.Context, vkID int64) (*repository.User, error)
	CreateWithReferralCode(ctx context.Context, vkID int64, username, firstName, referralCode string) (*repository.User, error)
}

type VKEvent struct {
	Type    string          `json:"type"`
	Object  json.RawMessage `json:"object"`
	GroupID int64           `json:"group_id"`
	Secret  string          `json:"secret"`
}

type MessageNewObject struct {
	Message VKMessage `json:"message"`
}

type VKMessage struct {
	ID          int64          `json:"id"`
	FromID      int64          `json:"from_id"`
	PeerID      int64          `json:"peer_id"`
	Text        string         `json:"text"`
	Ref         string         `json:"ref"`
	Attachments []VKAttachment `json:"attachments"`
	Payload     string         `json:"payload"`
}

type VKAttachment struct {
	Type  string  `json:"type"`
	Photo VKPhoto `json:"photo"`
}

type VKPhoto struct {
	Sizes     []VKPhotoSize `json:"sizes"`
	Photo75   string        `json:"photo_75"`
	Photo130  string        `json:"photo_130"`
	Photo604  string        `json:"photo_604"`
	Photo807  string        `json:"photo_807"`
	Photo1280 string        `json:"photo_1280"`
	Photo2560 string        `json:"photo_2560"`
	Src       string        `json:"src"`
	SrcBig    string        `json:"src_big"`
	SrcSmall  string        `json:"src_small"`
}

type VKPhotoSize struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type MessageEventObject struct {
	UserID  int64           `json:"user_id"`
	PeerID  int64           `json:"peer_id"`
	EventID string          `json:"event_id"`
	Payload json.RawMessage `json:"payload"`
}

type CallbackPayload struct {
	Type       string `json:"type"`
	TariffID   int    `json:"tariff_id,omitempty"`
	CategoryID int    `json:"category_id,omitempty"`
	PromptID   int    `json:"prompt_id,omitempty"`
	Page       int    `json:"page,omitempty"`
}

type Handler struct {
	state     flows.StateMgr
	sender    *Sender
	userRepo  userStore
	statsRepo *repository.StatsRepo
	activity  *repository.ActivityRepo
	registry  *flows.Registry
}

func NewHandler(
	state flows.StateMgr,
	sender *Sender,
	userRepo *repository.UserRepo,
	statsRepo *repository.StatsRepo,
	activityRepo *repository.ActivityRepo,
	reg *flows.Registry,
) *Handler {
	return &Handler{
		state:     state,
		sender:    sender,
		userRepo:  userRepo,
		statsRepo: statsRepo,
		activity:  activityRepo,
		registry:  reg,
	}
}

func (h *Handler) Handle(ctx context.Context, event *VKEvent) {
	switch event.Type {
	case "message_new":
		h.handleMessage(ctx, event.Object)
	case "message_event":
		h.handleCallback(ctx, event.Object)
	default:
		log.Debug().Str("type", event.Type).Msg("unknown VK event type")
	}
}

func (h *Handler) handleMessage(ctx context.Context, raw json.RawMessage) {
	var obj MessageNewObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		log.Error().Err(err).Msg("failed to parse message_new")
		return
	}

	msg := obj.Message
	vkID := msg.FromID
	if vkID <= 0 {
		return
	}

	user := h.ensureUser(ctx, vkID, "", "", msg.Ref)
	if user == nil {
		return
	}

	log.Info().Int64("vk_id", vkID).Str("text", msg.Text).Msg("incoming message")
	h.recordActivity(ctx, repository.ActivityEvent{
		UserVKID:  vkID,
		EventType: repository.ActivityEventMessageReceived,
		Meta: map[string]any{
			"has_payload": msg.Payload != "",
			"has_text":    msg.Text != "",
			"photo_count": len(msg.Attachments),
		},
	})

	state, err := h.state.Get(ctx, vkID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get state")
		return
	}

	if h.handleMessagePayload(ctx, msg, user, state) {
		return
	}

	fc := &flows.Context{
		VkID:    vkID,
		User:    toFlowUser(user),
		State:   state,
		Message: &flows.InMessage{Text: msg.Text, Photos: extractPhotos(msg.Attachments)},
	}

	h.registry.HandleMessage(ctx, fc)
}

func (h *Handler) handleCallback(ctx context.Context, raw json.RawMessage) {
	var obj MessageEventObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		log.Error().Err(err).Msg("failed to parse message_event")
		return
	}

	h.answerCallbackEvent(ctx, &obj)

	var cbPayload CallbackPayload
	if err := json.Unmarshal(obj.Payload, &cbPayload); err != nil || cbPayload.Type == "" {
		if err != nil {
			log.Error().Err(err).Msg("failed to parse callback payload")
		}
		return
	}

	vkID := obj.UserID
	user := h.ensureUser(ctx, vkID, "", "", "")
	if user == nil {
		return
	}

	log.Info().Int64("vk_id", vkID).Str("type", cbPayload.Type).Msg("callback button pressed")

	_ = h.statsRepo.RecordClick(ctx, vkID, cbPayload.Type)
	h.recordActivity(ctx, repository.ActivityEvent{
		UserVKID:  vkID,
		EventType: repository.ActivityEventButtonClick,
		ActionKey: cbPayload.Type,
	})

	state, _ := h.state.Get(ctx, vkID)

	fc := &flows.Context{
		VkID:  vkID,
		User:  toFlowUser(user),
		State: state,
		Callback: &flows.CallbackData{
			Type:       cbPayload.Type,
			EventID:    obj.EventID,
			TariffID:   cbPayload.TariffID,
			CategoryID: cbPayload.CategoryID,
			PromptID:   cbPayload.PromptID,
			Page:       cbPayload.Page,
		},
	}

	h.registry.HandleCallback(ctx, fc)
}

func (h *Handler) handleMessagePayload(ctx context.Context, msg VKMessage, user *repository.User, state *flows.State) bool {
	if msg.Payload == "" {
		return false
	}

	var cbPayload CallbackPayload
	if err := json.Unmarshal([]byte(msg.Payload), &cbPayload); err != nil || cbPayload.Type == "" {
		if err != nil {
			log.Debug().Err(err).Int64("vk_id", msg.FromID).Str("payload", msg.Payload).Msg("failed to parse message payload")
		}
		return false
	}

	log.Info().Int64("vk_id", msg.FromID).Str("type", cbPayload.Type).Msg("reply keyboard button pressed")
	_ = h.statsRepo.RecordClick(ctx, msg.FromID, cbPayload.Type)
	h.recordActivity(ctx, repository.ActivityEvent{
		UserVKID:  msg.FromID,
		EventType: repository.ActivityEventButtonClick,
		ActionKey: cbPayload.Type,
	})

	fc := &flows.Context{
		VkID:  msg.FromID,
		User:  toFlowUser(user),
		State: state,
		Callback: &flows.CallbackData{
			Type:       cbPayload.Type,
			TariffID:   cbPayload.TariffID,
			CategoryID: cbPayload.CategoryID,
			PromptID:   cbPayload.PromptID,
			Page:       cbPayload.Page,
		},
	}

	h.registry.HandleCallback(ctx, fc)
	return true
}

func (h *Handler) answerCallbackEvent(ctx context.Context, obj *MessageEventObject) {
	if obj == nil || obj.EventID == "" {
		return
	}
	if err := h.sender.vk.SendEventAnswer(ctx, obj.EventID, obj.UserID, obj.PeerID); err != nil {
		log.Warn().Err(err).Int64("vk_id", obj.UserID).Msg("failed to acknowledge callback event")
	}
}

func (h *Handler) ensureUser(ctx context.Context, vkID int64, username, firstName, referralCode string) *repository.User {
	user, err := h.userRepo.GetByVKID(ctx, vkID)
	if err != nil {
		log.Error().Err(err).Int64("vk_id", vkID).Msg("failed to fetch user")
		return nil
	}
	if user == nil {
		user, err = h.userRepo.CreateWithReferralCode(ctx, vkID, username, firstName, referralCode)
		if err != nil {
			log.Error().Err(err).Int64("vk_id", vkID).Msg("failed to create user")
			return nil
		}
		log.Info().Int64("vk_id", vkID).Msg("user created")
		h.recordActivity(ctx, repository.ActivityEvent{
			UserVKID:  vkID,
			EventType: repository.ActivityEventUserRegistered,
		})
		if user.ReferredBy != nil {
			h.recordActivity(ctx, repository.ActivityEvent{
				UserVKID:  vkID,
				EventType: repository.ActivityEventReferralCreated,
				ActionKey: "referral",
				ScreenKey: "welcome",
				Meta: map[string]any{
					"referrer_vk_id": *user.ReferredBy,
				},
			})
		}
	}
	return user
}

func (h *Handler) recordActivity(ctx context.Context, event repository.ActivityEvent) {
	if h.activity == nil {
		return
	}
	if err := h.activity.Record(ctx, event); err != nil {
		log.Debug().Err(err).Int64("vk_id", event.UserVKID).Str("event_type", event.EventType).Msg("failed to record activity")
	}
}

func extractPhotos(attachments []VKAttachment) []string {
	var urls []string
	for _, a := range attachments {
		if a.Type != "photo" {
			continue
		}

		if best := extractBestPhotoURL(a.Photo); best != "" {
			urls = append(urls, best)
		}
	}
	return urls
}

func extractBestPhotoURL(photo VKPhoto) string {
	best := ""
	bestArea := 0
	for _, sz := range photo.Sizes {
		if sz.URL == "" {
			continue
		}
		area := sz.Width * sz.Height
		if area == 0 {
			area = sz.Width
		}
		if best == "" || area > bestArea {
			best = sz.URL
			bestArea = area
		}
	}
	if best != "" {
		return best
	}

	for _, url := range []string{
		photo.Photo2560,
		photo.Photo1280,
		photo.Photo807,
		photo.Photo604,
		photo.SrcBig,
		photo.Src,
		photo.Photo130,
		photo.Photo75,
		photo.SrcSmall,
	} {
		if url != "" {
			return url
		}
	}
	return ""
}

func toFlowUser(u *repository.User) *flows.User {
	return &flows.User{
		VKID:            u.VKID,
		Gender:          u.Gender,
		FreeGens:        u.FreeGens,
		PaidGens:        u.PaidGens,
		Status:          u.Status,
		ReferralCode:    u.ReferralCode,
		SavedPhotoURL:   u.SavedPhotoURL,
		UseSavedPhoto:   u.UseSavedPhoto,
		PrefModel:       u.PrefModel,
		PrefResolution:  u.PrefResolution,
		PrefAspectRatio: u.PrefAspectRatio,
	}
}

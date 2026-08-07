package flows

import (
	"context"

	"github.com/hibiken/asynq"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/vkgroup"
	"vk_neuro_bot/internal/wavespeed"
	"vk_neuro_bot/internal/yukassa"
)

// ─── Shared types ────────────────────────────────────────────────────────────

const (
	StepWelcome                = "welcome"
	StepFreeGenStart           = "free_gen_start"
	StepAwaitingGender         = "awaiting_gender"
	StepAwaitingPhoto          = "awaiting_photo"
	StepAwaitingPrompt         = "awaiting_prompt"
	StepAwaitingPhotoEdit      = "awaiting_photo_edit"
	StepAwaitingEditPrompt     = "awaiting_edit_prompt"
	StepAwaitingResultEdit     = "awaiting_result_edit"
	StepMainMenu               = "main_menu"
	StepAfterGen               = "after_gen"
	StepTariffs                = "tariffs"
	StepSettings               = "settings"
	StepCoupleMenu             = "couple_menu"
	StepAwaitingSavedPhoto     = "awaiting_saved_photo"
	StepReadyPromptsCategories = "ready_prompts_categories"
	StepReadyPromptsPrompts    = "ready_prompts_prompts"
	StepCoupleAwaitingPhoto    = "couple_awaiting_photo"
	StepCoupleCategories       = "couple_categories"
	StepCouplePrompts          = "couple_prompts"
)

type User struct {
	VKID                  int64
	Gender                string
	FreeGens              int
	PaidGens              int
	Status                string
	ReferralCode          string
	SavedPhotoURLs        []string
	SavedPhotoAttachments []string
	UseSavedPhoto         bool
	PrefModel             string
	PrefResolution        string
	PrefAspectRatio       string
}

func (u *User) TotalGens() int {
	if u == nil {
		return 0
	}
	return u.FreeGens + u.PaidGens
}

func (u *User) HasGens() bool { return u.TotalGens() > 0 }

// copyPrefs копирует Model, Resolution, AspectRatio из src в dst и возвращает dst.
func copyPrefs(dst *State, src *State) *State {
	if src == nil {
		return dst
	}
	if dst.Model == "" {
		dst.Model = src.Model
	}
	if dst.Resolution == "" {
		dst.Resolution = src.Resolution
	}
	if dst.AspectRatio == "" {
		dst.AspectRatio = src.AspectRatio
	}
	// Парные фото загружаются до выбора категории, поэтому переносим их через
	// промежуточные шаги выбора. Поле читает только couple-сценарий.
	if len(dst.CouplePhotoURLs) == 0 && len(src.CouplePhotoURLs) > 0 {
		dst.CouplePhotoURLs = clonePhotoURLs(src.CouplePhotoURLs)
	}
	return dst
}

type State struct {
	Step       string `json:"step"`
	PrevStep   string `json:"prev_step,omitempty"`
	PromptType string `json:"prompt_type,omitempty"`
	TemplateID int    `json:"template_id,omitempty"`
	CategoryID int    `json:"category_id,omitempty"`
	// Section — раздел дерева, в котором сейчас находится пользователь,
	// SectionID — узел, чьи дети показаны (0 = корень раздела).
	Section         string   `json:"section,omitempty"`
	SectionID       int      `json:"section_id,omitempty"`
	CategoryPage    int      `json:"category_page,omitempty"`
	PromptPage      int      `json:"prompt_page,omitempty"`
	GenerationID    int64    `json:"generation_id,omitempty"`
	Model           string   `json:"model,omitempty"`
	CustomPrompt    string   `json:"custom_prompt,omitempty"`
	PhotoURL        string   `json:"photo_url,omitempty"`
	InputPhotoURLs  []string `json:"input_photo_urls,omitempty"`
	CouplePhotoURLs []string `json:"couple_photo_urls,omitempty"`
	PhotoBatchID    string   `json:"photo_batch_id,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	AspectRatio     string   `json:"aspect_ratio,omitempty"`
}

type InMessage struct {
	Text             string
	Photos           []string
	PhotoAttachments []string // VK attachment strings вида photo{owner_id}_{id}
}

type CallbackData struct {
	Type       string
	EventID    string
	TariffID   int
	CategoryID int
	PromptID   int
	Page       int
}

type Context struct {
	VkID     int64
	User     *User
	State    *State
	Message  *InMessage
	Callback *CallbackData
}

type ScreenMessage struct {
	Key         string
	Text        string
	ImageURL    *string
	Attachments []string // готовые VK attachment strings (joined в SendScreen через запятую)
	Keyboard    string
	CacheKey    string
}

// PhotoStorage — интерфейс S3-хранилища фото.
type PhotoStorage interface {
	UploadFromURL(ctx context.Context, key, url string) (string, error)
	PublicURL(key string) string
}

// ─── Interfaces ───────────────────────────────────────────────────────────────

// Sender — интерфейс отправки сообщений (реализует bot.Sender).
type Sender interface {
	SendMsg(ctx context.Context, vkID int64, key string, kbJSON string) error
	SendText(ctx context.Context, vkID int64, text string, kbJSON string) error
	SendPhoto(ctx context.Context, vkID int64, photoURL, caption, kbJSON string) error
	SendScreen(ctx context.Context, vkID int64, screen *ScreenMessage) error
	SendScreenText(ctx context.Context, vkID int64, key string, data map[string]any) error
	SendPhotoResult(ctx context.Context, vkID int64, photoURL, model, resolution, aspectRatio string) error
}

// StateMgr — интерфейс управления состоянием (реализует bot.StateManager).
type StateMgr interface {
	Get(ctx context.Context, vkID int64) (*State, error)
	Set(ctx context.Context, vkID int64, st *State) error
	SetStep(ctx context.Context, vkID int64, step string) error
	Reset(ctx context.Context, vkID int64) error
}

type MessageReader interface {
	Get(ctx context.Context, key string) (*repository.Message, error)
}

type CategoryReader interface {
	ListReadyPromptCategories(ctx context.Context, gender string) ([]*repository.Category, error)
	ListActive(ctx context.Context, gender string) ([]*repository.Category, error)
	ListActiveCouple(ctx context.Context) ([]*repository.Category, error)
	// Обход дерева разделов.
	ListRoots(ctx context.Context, section string, filter repository.CategoryFilter) ([]*repository.Category, error)
	ListChildren(ctx context.Context, parentID int, filter repository.CategoryFilter) ([]*repository.Category, error)
	Path(ctx context.Context, id int) ([]*repository.Category, error)
	GetByID(ctx context.Context, id int) (*repository.Category, error)
}

type PromptReader interface {
	ListByCategory(ctx context.Context, categoryID int, gender string) ([]*repository.Prompt, error)
	GetByID(ctx context.Context, id int) (*repository.Prompt, error)
}

type TariffReader interface {
	ListActive(ctx context.Context) ([]*repository.Tariff, error)
	GetByID(ctx context.Context, id int) (*repository.Tariff, error)
}

type UserStore interface {
	GetByVKID(ctx context.Context, vkID int64) (*repository.User, error)
	SetGender(ctx context.Context, vkID int64, gender string) error
	SetSubscribed(ctx context.Context, vkID int64, subscribed bool) error
	SetSavedPhotos(ctx context.Context, vkID int64, urls []string, attachments []string) error
	SetUseSavedPhoto(ctx context.Context, vkID int64, enabled bool) error
	SaveSettings(ctx context.Context, vkID int64, model, resolution, aspectRatio string) error
}

type OrderStore interface {
	Create(ctx context.Context, userVKID int64, tariffID int, amount float64) (*repository.Order, error)
	SetPaymentID(ctx context.Context, orderID int64, paymentID string) error
	SetPaymentLink(ctx context.Context, orderID int64, paymentID, paymentURL, payToken string) error
	GetByPayToken(ctx context.Context, payToken string) (*repository.Order, error)
	SettleSuccessfulPayment(ctx context.Context, paymentID string, paidGensHint int) (*repository.PaymentSettlementResult, error)
	CancelPayment(ctx context.Context, paymentID string) (*repository.PaymentCancellationResult, error)
}

// ─── Deps ────────────────────────────────────────────────────────────────────

type Deps struct {
	Sender        Sender
	State         StateMgr
	UserRepo      UserStore
	GenRepo       *repository.GenerationRepo
	TariffRepo    TariffReader
	OrderRepo     OrderStore
	MsgRepo       MessageReader
	CatRepo       CategoryReader
	PromptRepo    PromptReader
	RefRepo       *repository.ReferralRepo
	StatsRepo     *repository.StatsRepo
	ActivityRepo  *repository.ActivityRepo
	AsynqClient   *asynq.Client
	WaveSpeed     *wavespeed.Client
	Yukassa       *yukassa.Client
	VKClient      *vkgroup.Client
	VKGroupID     int64
	VKGroupURL    string
	DefaultModel  string
	PublicBaseURL string
	Storage       PhotoStorage
}

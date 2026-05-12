package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/repository"
)

const usersPageSize = 24

type UsersHandler struct {
	users  *repository.UserRepo
	orders *repository.OrderRepo
	rdb    *redis.Client
	tmpl   *template.Template
}

type usersPageData struct {
	Title          string
	Active         string
	AdminBase      string
	Summary        usersSummaryView
	Query          string
	Status         string
	StatusLabel    string
	Users          []userListItemView
	Total          int
	VisibleCount   int
	Page           int
	HasPrev        bool
	HasNext        bool
	PrevURL        string
	NextURL        string
	FiltersApplied bool
	Detail         *userDetailPageView
}

type usersSummaryView struct {
	TotalUsers    int
	PaidUsers     int
	NewUsersToday int
	ActiveUsers7d int
}

type userListItemView struct {
	VKID            int64
	DisplayName     string
	Subtitle        string
	Username        string
	ProfileURL      string
	StatusLabel     string
	StatusClass     string
	TotalGens       int
	FreeGens        int
	PaidGens        int
	CreatedAt       string
	LastActivity    string
	HasSavedPhoto   bool
	HasSubscription bool
	DetailURL       string
}

type userDetailPageView struct {
	VKID               int64
	DisplayName        string
	Subtitle           string
	ProfileURL         string
	StatusLabel        string
	StatusClass        string
	FreeGens           int
	PaidGens           int
	TotalGens          int
	GeneratedCount     int
	CompletedGenCount  int
	ReferredCount      int
	ReferralBonusCount int
	OrdersCount        int
	SuccessOrdersCount int
	TotalSpent         string
	CreatedAt          string
	LastActivity       string
	Gender             string
	ReferralCode       string
	SubscribedLabel    string
	SavedPhotoLabel    string
	SavedPhotoURL      string
	PrefModel          string
	PrefResolution     string
	PrefAspectRatio    string
	Orders             []userOrderView
	AddGensURL         string
	DeleteURL          string
	BackURL            string
}

type userOrderView struct {
	ID          int64
	TariffID    int
	Amount      string
	StatusLabel string
	StatusClass string
	CreatedAt   string
}

func NewUsersHandler(users *repository.UserRepo, orders *repository.OrderRepo, rdb *redis.Client) *UsersHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/users.html")
	return &UsersHandler{users: users, orders: orders, rdb: rdb, tmpl: tmpl}
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	base := GetAdminBase(r)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	params := repository.AdminUserListParams{
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		Status: normalizeUserStatus(r.URL.Query().Get("status")),
		Limit:  usersPageSize,
		Offset: (page - 1) * usersPageSize,
	}

	users, err := h.users.AdminList(r.Context(), params)
	if err != nil {
		log.Error().Err(err).Msg("failed to load admin users list")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	total, err := h.users.AdminCount(r.Context(), params)
	if err != nil {
		log.Error().Err(err).Msg("failed to count admin users")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	summary, err := h.users.AdminSummary(r.Context(), time.Now().UTC())
	if err != nil {
		log.Error().Err(err).Msg("failed to build users summary")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	views := make([]userListItemView, 0, len(users))
	for _, item := range users {
		views = append(views, buildUserListItemView(item, base))
	}

	data := usersPageData{
		Title:          "Пользователи",
		Active:         "users",
		AdminBase:      base,
		Summary:        buildUsersSummaryView(summary),
		Query:          params.Query,
		Status:         params.Status,
		StatusLabel:    userStatusLabel(params.Status),
		Users:          views,
		Total:          total,
		VisibleCount:   len(views),
		Page:           page,
		HasPrev:        page > 1,
		HasNext:        params.Offset+usersPageSize < total,
		PrevURL:        buildUsersListURL(base, page-1, params.Query, params.Status),
		NextURL:        buildUsersListURL(base, page+1, params.Query, params.Status),
		FiltersApplied: params.Query != "" || params.Status != "",
	}

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("failed to render users template")
	}
}

func (h *UsersHandler) AddGens(w http.ResponseWriter, r *http.Request) {
	vkIDStr := chi.URLParam(r, "id")
	vkID, err := strconv.ParseInt(vkIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	count, err := strconv.Atoi(r.FormValue("count"))
	if err != nil || count <= 0 {
		http.Redirect(w, r, GetAdminBase(r)+"/users/"+vkIDStr, http.StatusSeeOther)
		return
	}

	genType := r.FormValue("type")
	if genType == "paid" {
		err = h.users.AddPaidGens(r.Context(), vkID, count)
	} else {
		err = h.users.AddFreeGens(r.Context(), vkID, count)
	}
	if err != nil {
		log.Error().Err(err).Msg("failed to add generations")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, GetAdminBase(r)+"/users/"+vkIDStr, http.StatusSeeOther)
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.users.Delete(r.Context(), vkID); err != nil {
		log.Error().Err(err).Int64("vk_id", vkID).Msg("failed to delete user")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = h.rdb.Del(r.Context(), fmt.Sprintf("state:%d", vkID))
	http.Redirect(w, r, GetAdminBase(r)+"/users", http.StatusSeeOther)
}

func (h *UsersHandler) Detail(w http.ResponseWriter, r *http.Request) {
	vkIDStr := chi.URLParam(r, "id")
	vkID, err := strconv.ParseInt(vkIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	user, err := h.users.AdminDetail(r.Context(), vkID)
	if err != nil {
		log.Error().Err(err).Int64("vk_id", vkID).Msg("failed to load user detail")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	orders, err := h.orders.ListByUser(r.Context(), vkID)
	if err != nil {
		log.Error().Err(err).Int64("vk_id", vkID).Msg("failed to load user orders")
		orders = nil
	}

	base := GetAdminBase(r)
	data := usersPageData{
		Title:     "Пользователь",
		Active:    "users",
		AdminBase: base,
		Detail:    buildUserDetailPageView(user, orders, base),
	}

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("failed to render user detail template")
	}
}

func buildUsersSummaryView(summary *repository.AdminUserSummary) usersSummaryView {
	if summary == nil {
		return usersSummaryView{}
	}
	return usersSummaryView{
		TotalUsers:    summary.TotalUsers,
		PaidUsers:     summary.PaidUsers,
		NewUsersToday: summary.NewUsersToday,
		ActiveUsers7d: summary.ActiveUsers7d,
	}
}

func buildUserListItemView(user *repository.AdminUserListItem, base string) userListItemView {
	statusLabel, statusClass := userStatusBadge(user.Status)
	return userListItemView{
		VKID:            user.VKID,
		DisplayName:     userDisplayName(user.FirstName, user.Username, user.VKID),
		Subtitle:        userSubtitle(user.Username, user.VKID),
		Username:        user.Username,
		ProfileURL:      vkProfileURL(user.VKID),
		StatusLabel:     statusLabel,
		StatusClass:     statusClass,
		TotalGens:       user.FreeGens + user.PaidGens,
		FreeGens:        user.FreeGens,
		PaidGens:        user.PaidGens,
		CreatedAt:       formatDateTime(user.CreatedAt),
		LastActivity:    formatDateTime(user.LastActivity),
		HasSavedPhoto:   len(user.SavedPhotoURLs) > 0,
		HasSubscription: user.Subscribed,
		DetailURL:       fmt.Sprintf("%s/users/%d", base, user.VKID),
	}
}

func buildUserDetailPageView(user *repository.AdminUserDetail, orders []*repository.Order, base string) *userDetailPageView {
	if user == nil {
		return nil
	}

	orderViews := make([]userOrderView, 0, len(orders))
	for _, order := range orders {
		orderViews = append(orderViews, userOrderView{
			ID:          order.ID,
			TariffID:    order.TariffID,
			Amount:      fmt.Sprintf("%.0f ₽", order.Amount),
			StatusLabel: orderStatusLabel(order.Status),
			StatusClass: orderStatusClass(order.Status),
			CreatedAt:   formatDateTime(order.CreatedAt),
		})
	}

	statusLabel, statusClass := userStatusBadge(user.Status)
	savedPhotoURL := ""
	if len(user.SavedPhotoURLs) > 0 {
		savedPhotoURL = user.SavedPhotoURLs[0]
	}

	return &userDetailPageView{
		VKID:               user.VKID,
		DisplayName:        userDisplayName(user.FirstName, user.Username, user.VKID),
		Subtitle:           userSubtitle(user.Username, user.VKID),
		ProfileURL:         vkProfileURL(user.VKID),
		StatusLabel:        statusLabel,
		StatusClass:        statusClass,
		FreeGens:           user.FreeGens,
		PaidGens:           user.PaidGens,
		TotalGens:          user.FreeGens + user.PaidGens,
		GeneratedCount:     user.GeneratedCount,
		CompletedGenCount:  user.CompletedGenCount,
		ReferredCount:      user.ReferredCount,
		ReferralBonusCount: user.ReferralBonusCount,
		OrdersCount:        user.OrdersCount,
		SuccessOrdersCount: user.SuccessOrdersCount,
		TotalSpent:         fmt.Sprintf("%.0f ₽", user.TotalSpent),
		CreatedAt:          formatDateTime(user.CreatedAt),
		LastActivity:       formatDateTime(user.LastActivity),
		Gender:             genderLabel(user.Gender),
		ReferralCode:       user.ReferralCode,
		SubscribedLabel:    booleanLabel(user.Subscribed, "Подписан", "Не подписан"),
		SavedPhotoLabel:    savedPhotoState(user.SavedPhotoURLs, user.UseSavedPhoto),
		SavedPhotoURL:      savedPhotoURL,
		PrefModel:          flows.ModelDisplayName(user.PrefModel),
		PrefResolution:     fallbackValue(user.PrefResolution, "1k"),
		PrefAspectRatio:    fallbackValue(user.PrefAspectRatio, "авто"),
		Orders:             orderViews,
		AddGensURL:         fmt.Sprintf("%s/users/%d/add-gens", base, user.VKID),
		DeleteURL:          fmt.Sprintf("%s/users/%d/delete", base, user.VKID),
		BackURL:            base + "/users",
	}
}

func buildUsersListURL(base string, page int, query, status string) string {
	if page < 1 {
		page = 1
	}
	values := url.Values{}
	values.Set("page", strconv.Itoa(page))
	if strings.TrimSpace(query) != "" {
		values.Set("q", strings.TrimSpace(query))
	}
	if normalizeUserStatus(status) != "" {
		values.Set("status", normalizeUserStatus(status))
	}
	return base + "/users?" + values.Encode()
}

func normalizeUserStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "free", "paid":
		return strings.TrimSpace(strings.ToLower(status))
	default:
		return ""
	}
}

func userStatusLabel(status string) string {
	switch normalizeUserStatus(status) {
	case "paid":
		return "Только платящие"
	case "free":
		return "Только free"
	default:
		return "Все статусы"
	}
}

func userStatusBadge(status string) (string, string) {
	if status == "paid" {
		return "paid", "success"
	}
	return "free", "secondary"
}

func userDisplayName(firstName, username string, vkID int64) string {
	firstName = strings.TrimSpace(firstName)
	username = strings.TrimSpace(username)
	switch {
	case firstName != "":
		return firstName
	case username != "":
		return "@" + username
	default:
		return fmt.Sprintf("Пользователь %d", vkID)
	}
}

func userSubtitle(username string, vkID int64) string {
	username = strings.TrimSpace(username)
	if username != "" {
		return fmt.Sprintf("@%s · vk.com/id%d", username, vkID)
	}
	return fmt.Sprintf("vk.com/id%d", vkID)
}

func vkProfileURL(vkID int64) string {
	return fmt.Sprintf("https://vk.com/id%d", vkID)
}

func genderLabel(gender string) string {
	switch gender {
	case "male":
		return "Мужской"
	case "female":
		return "Женский"
	default:
		return "Не указан"
	}
}

func booleanLabel(value bool, yes, no string) string {
	if value {
		return yes
	}
	return no
}

func savedPhotoState(urls []string, enabled bool) string {
	if len(urls) == 0 {
		return "Нет сохранённого фото"
	}
	if enabled {
		return "Сохранено и включено"
	}
	return "Сохранено, выключено"
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func orderStatusLabel(status string) string {
	switch status {
	case "succeeded":
		return "Успешно"
	case "failed":
		return "Ошибка"
	default:
		return "Ожидает"
	}
}

func orderStatusClass(status string) string {
	switch status {
	case "succeeded":
		return "success"
	case "failed":
		return "danger"
	default:
		return "warning"
	}
}

func formatDateTime(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Local().Format("02.01.2006 15:04")
}

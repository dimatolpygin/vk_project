package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

type UsersHandler struct {
	users  *repository.UserRepo
	orders *repository.OrderRepo
	rdb    *redis.Client
	tmpl   *template.Template
}

func NewUsersHandler(users *repository.UserRepo, orders *repository.OrderRepo, rdb *redis.Client) *UsersHandler {
	tmpl := parseTemplates("templates/layout.html", "templates/users.html")
	return &UsersHandler{users: users, orders: orders, rdb: rdb, tmpl: tmpl}
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	users, err := h.users.List(r.Context(), limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения пользователей")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	total, _ := h.users.Count(r.Context())

	data := map[string]any{
		"Title":   "Пользователи",
		"Active":  "users",
		"Users":   users,
		"Total":   total,
		"Page":    page,
		"HasPrev": page > 1,
		"HasNext": offset+limit < total,
	}

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("ошибка рендеринга шаблона users")
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
		http.Redirect(w, r, "/admin/users/"+vkIDStr, http.StatusSeeOther)
		return
	}

	genType := r.FormValue("type")
	if genType == "paid" {
		err = h.users.AddPaidGens(r.Context(), vkID, count)
	} else {
		err = h.users.AddFreeGens(r.Context(), vkID, count)
	}
	if err != nil {
		log.Error().Err(err).Msg("ошибка начисления генераций")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users/"+vkIDStr, http.StatusSeeOther)
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.users.Delete(r.Context(), vkID); err != nil {
		log.Error().Err(err).Int64("vk_id", vkID).Msg("ошибка удаления пользователя")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = h.rdb.Del(r.Context(), fmt.Sprintf("state:%d", vkID))
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *UsersHandler) Detail(w http.ResponseWriter, r *http.Request) {
	vkIDStr := chi.URLParam(r, "id")
	vkID, err := strconv.ParseInt(vkIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	user, err := h.users.GetByVKID(r.Context(), vkID)
	if err != nil || user == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	orders, err := h.orders.ListByUser(r.Context(), vkID)
	if err != nil {
		orders = nil
	}

	data := map[string]any{
		"Title":  "Пользователь",
		"Active": "users",
		"User":   user,
		"Orders": orders,
	}

	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("ошибка рендеринга шаблона user detail")
	}
}

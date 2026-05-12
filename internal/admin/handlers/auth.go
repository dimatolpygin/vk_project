package handlers

import (
	"html/template"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"vk_neuro_bot/internal/repository"
)

type AuthHandler struct {
	db           *pgxpool.Pool
	envLogin     string
	envPassword  string
	sessions     *SessionStore
	loginTmpl    *template.Template
	changePwTmpl *template.Template
}

func NewAuthHandler(db *pgxpool.Pool, envLogin, envPassword string, sessions *SessionStore) *AuthHandler {
	loginTmpl := template.Must(template.New("").ParseFiles("templates/login.html"))
	changePwTmpl := parseTemplates("templates/layout.html", "templates/change_password.html")
	return &AuthHandler{
		db:           db,
		envLogin:     envLogin,
		envPassword:  envPassword,
		sessions:     sessions,
		loginTmpl:    loginTmpl,
		changePwTmpl: changePwTmpl,
	}
}

func (h *AuthHandler) LoginGet(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{"Error": r.URL.Query().Get("error") != ""}
	if err := h.loginTmpl.ExecuteTemplate(w, "login", data); err != nil {
		log.Error().Err(err).Msg("render login")
	}
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	login := r.FormValue("login")
	password := r.FormValue("password")

	if login != h.envLogin || !h.checkPassword(r, password) {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusSeeOther)
		return
	}

	token := h.sessions.Create()
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("admin_session"); err == nil {
		h.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

type changePasswordData struct {
	Title  string
	Active string
	Error  string
	OK     bool
}

func (h *AuthHandler) ChangePasswordGet(w http.ResponseWriter, r *http.Request) {
	data := changePasswordData{Title: "Сменить пароль"}
	if err := h.changePwTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("render change-password")
	}
}

func (h *AuthHandler) ChangePasswordPost(w http.ResponseWriter, r *http.Request) {
	current := r.FormValue("current_password")
	newPw := r.FormValue("new_password")
	confirm := r.FormValue("confirm_password")

	render := func(errMsg string, ok bool) {
		data := changePasswordData{Title: "Сменить пароль", Error: errMsg, OK: ok}
		if err := h.changePwTmpl.ExecuteTemplate(w, "layout", data); err != nil {
			log.Error().Err(err).Msg("render change-password")
		}
	}

	if !h.checkPassword(r, current) {
		render("Текущий пароль неверен", false)
		return
	}
	if newPw == "" {
		render("Новый пароль не может быть пустым", false)
		return
	}
	if newPw != confirm {
		render("Пароли не совпадают", false)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("bcrypt generate")
		render("Внутренняя ошибка", false)
		return
	}
	if err := repository.SetAdminConfig(r.Context(), h.db, "admin_password", string(hash)); err != nil {
		log.Error().Err(err).Msg("save admin_password")
		render("Ошибка сохранения", false)
		return
	}
	render("", true)
}

func (h *AuthHandler) checkPassword(r *http.Request, input string) bool {
	customHash, err := repository.GetAdminConfig(r.Context(), h.db, "admin_password")
	if err != nil {
		log.Error().Err(err).Msg("get admin_password config")
	}
	if customHash != "" {
		return bcrypt.CompareHashAndPassword([]byte(customHash), []byte(input)) == nil
	}
	return input == h.envPassword
}

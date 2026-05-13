package admin

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"vk_neuro_bot/internal/admin/handlers"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/repository"
)

// Storage — интерфейс S3-хранилища, используется для загрузки файлов из админки.
type Storage interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
	PublicURL(key string) string
}

type Server struct {
	router *chi.Mux
}

func NewServer(
	login, password string,
	db *pgxpool.Pool,
	users *repository.UserRepo,
	tariffs *repository.TariffRepo,
	msgs *repository.MessageRepo,
	broadcasts *repository.BroadcastRepo,
	cats *repository.CategoryRepo,
	prompts *repository.PromptRepo,
	stats *repository.StatsRepo,
	orders *repository.OrderRepo,
	rdb *redis.Client,
	asynqClient *asynq.Client,
	storage Storage,
	flowDeps *flows.Deps,
) *Server {
	s := &Server{}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(noCacheMiddleware)

	sessions := handlers.NewSessionStore()

	uh := handlers.NewUsersHandler(users, orders, rdb)
	th := handlers.NewTariffsHandler(tariffs)
	mh := handlers.NewMessagesHandler(msgs)
	bh := handlers.NewBroadcastsHandler(broadcasts, asynqClient)
	ch := handlers.NewCategoriesHandler(cats, prompts)
	sh := handlers.NewStatsHandler(stats)
	uploadH := handlers.NewUploadHandler(storage)
	ah := handlers.NewAuthHandler(db, login, password, sessions)
	ph := handlers.NewPaymentsHandler(flowDeps)

	// Public: форма входа (без авторизации)
	r.Get("/admin/login", ah.LoginGet)
	r.Post("/admin/login", ah.LoginPost)

	// Защищённые маршруты /admin (session cookie)
	r.Group(func(r chi.Router) {
		r.Use(sessionMiddleware(sessions))
		r.Use(adminBaseCtx("/admin"))
		r.Route("/admin", func(r chi.Router) {
			r.Get("/", sh.GetStats)
			r.Get("/stats", sh.GetStats)

			r.Post("/logout", ah.Logout)
			r.Get("/change-password", ah.ChangePasswordGet)
			r.Post("/change-password", ah.ChangePasswordPost)

			r.Post("/upload", uploadH.Upload)

			r.Get("/users", uh.List)
			r.Get("/users/{id}", uh.Detail)
			r.Post("/users/{id}/add-gens", uh.AddGens)
			r.Post("/users/{id}/delete", uh.Delete)

			r.Get("/tariffs", th.List)
			r.Post("/tariffs", th.Create)
			r.Put("/tariffs/{id}", th.Update)
			r.Delete("/tariffs/{id}", th.Delete)

			r.Get("/messages", mh.List)
			r.Post("/messages", mh.Upsert)
			r.Get("/messages/{key}", mh.Get)

			r.Get("/broadcasts", bh.List)
			r.Post("/broadcasts", bh.Create)
			r.Get("/broadcasts/{id}", bh.Get)

			r.Get("/categories", ch.ListCategories)
			r.Post("/categories", ch.CreateCategory)
			r.Put("/categories/{id}", ch.UpdateCategory)
			r.Delete("/categories/{id}", ch.DeleteCategory)
			r.Get("/categories/{id}/prompts", ch.ListPrompts)
			r.Post("/categories/{id}/prompts", ch.CreatePrompt)
			r.Put("/prompts/{id}", ch.UpdatePrompt)
			r.Delete("/prompts/{id}", ch.DeletePrompt)

			r.Post("/payment/settle", ph.Settle)
		})
	})

	// Суперадмин: Basic Auth из env (без смены пароля и логаута)
	r.Group(func(r chi.Router) {
		r.Use(basicAuth(login, password))
		r.Use(adminBaseCtx("/superadmin6736/admin"))
		r.Route("/superadmin6736/admin", func(r chi.Router) {
			r.Get("/", sh.GetStats)
			r.Get("/stats", sh.GetStats)

			r.Post("/upload", uploadH.Upload)

			r.Get("/users", uh.List)
			r.Get("/users/{id}", uh.Detail)
			r.Post("/users/{id}/add-gens", uh.AddGens)
			r.Post("/users/{id}/delete", uh.Delete)

			r.Get("/tariffs", th.List)
			r.Post("/tariffs", th.Create)
			r.Put("/tariffs/{id}", th.Update)
			r.Delete("/tariffs/{id}", th.Delete)

			r.Get("/messages", mh.List)
			r.Post("/messages", mh.Upsert)
			r.Get("/messages/{key}", mh.Get)

			r.Get("/broadcasts", bh.List)
			r.Post("/broadcasts", bh.Create)
			r.Get("/broadcasts/{id}", bh.Get)

			r.Get("/categories", ch.ListCategories)
			r.Post("/categories", ch.CreateCategory)
			r.Put("/categories/{id}", ch.UpdateCategory)
			r.Delete("/categories/{id}", ch.DeleteCategory)
			r.Get("/categories/{id}/prompts", ch.ListPrompts)
			r.Post("/categories/{id}/prompts", ch.CreatePrompt)
			r.Put("/prompts/{id}", ch.UpdatePrompt)
			r.Delete("/prompts/{id}", ch.DeletePrompt)

			r.Post("/payment/settle", ph.Settle)
		})
	})

	r.Get("/", sh.GetStats)

	s.router = r
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func adminBaseCtx(base string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), handlers.AdminBaseKey, base)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func sessionMiddleware(sessions *handlers.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("admin_session")
			if err != nil || !sessions.Valid(cookie.Value) {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func basicAuth(login, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok || user != login || pass != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="Admin"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

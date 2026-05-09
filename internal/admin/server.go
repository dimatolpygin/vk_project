package admin

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"vk_neuro_bot/internal/admin/handlers"
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
) *Server {
	s := &Server{}
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(basicAuth(login, password))

	uh := handlers.NewUsersHandler(users, orders, rdb)
	th := handlers.NewTariffsHandler(tariffs)
	mh := handlers.NewMessagesHandler(msgs)
	bh := handlers.NewBroadcastsHandler(broadcasts, asynqClient)
	ch := handlers.NewCategoriesHandler(cats, prompts)
	sh := handlers.NewStatsHandler(stats)
	uploadH := handlers.NewUploadHandler(storage)

	r.Get("/", sh.GetStats)

	r.Route("/admin", func(r chi.Router) {
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
	})

	s.router = r
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
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

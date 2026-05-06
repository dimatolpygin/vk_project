package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"vk_neuro_bot/internal/admin/handlers"
	"vk_neuro_bot/internal/repository"
)

type Server struct {
	router *chi.Mux
}

func NewServer(
	login, password string,
	users *repository.UserRepo,
	tariffs *repository.TariffRepo,
	msgs *repository.MessageRepo,
	cats *repository.CategoryRepo,
	prompts *repository.PromptRepo,
	stats *repository.StatsRepo,
	orders *repository.OrderRepo,
) *Server {
	s := &Server{}
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(basicAuth(login, password))

	uh := handlers.NewUsersHandler(users, orders)
	th := handlers.NewTariffsHandler(tariffs)
	mh := handlers.NewMessagesHandler(msgs)
	ch := handlers.NewCategoriesHandler(cats, prompts)
	sh := handlers.NewStatsHandler(stats)

	r.Get("/", sh.GetStats)
	r.Get("/admin", sh.GetStats)

	r.Route("/admin", func(r chi.Router) {
		r.Get("/stats", sh.GetStats)

		r.Get("/users", uh.List)
		r.Get("/users/{id}", uh.Detail)

		r.Get("/tariffs", th.List)
		r.Post("/tariffs", th.Create)
		r.Put("/tariffs/{id}", th.Update)
		r.Delete("/tariffs/{id}", th.Delete)

		r.Get("/messages", mh.List)
		r.Post("/messages", mh.Upsert)
		r.Get("/messages/{key}", mh.Get)

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

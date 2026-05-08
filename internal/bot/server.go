package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/config"
	"vk_neuro_bot/internal/yukassa"
)

type Server struct {
	cfg      *config.Config
	handler  *Handler
	yukassa  *yukassa.Client
	flowDeps *flows.Deps
	router   *chi.Mux
}

func NewServer(cfg *config.Config, handler *Handler, yk *yukassa.Client, d *flows.Deps) *Server {
	s := &Server{cfg: cfg, handler: handler, yukassa: yk, flowDeps: d}
	s.router = chi.NewRouter()
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RealIP)
	s.setupRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) setupRoutes() {
	s.router.Post("/vk/webhook", s.handleVKWebhook)
	s.router.Post("/webhook/yukassa", s.handleYukassaWebhook)
	s.router.Post("/webhook/wavespeed", s.handleWavespeedWebhook)
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func (s *Server) handleVKWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var event VKEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Error().Err(err).Msg("не удалось распарсить VK event")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if s.cfg.VKSecret != "" && event.Secret != s.cfg.VKSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if event.Type == "confirmation" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, s.cfg.VKConfirmationToken)
		return
	}

	go s.handler.Handle(context.WithoutCancel(r.Context()), &event)

	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, "ok")
}

func (s *Server) handleYukassaWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Payment-Sha256-Signature")
	if err := s.yukassa.VerifyWebhookSignature(sig, body); err != nil {
		log.Error().Err(err).Msg("неверная подпись ЮKassa webhook")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	event, err := s.yukassa.ParseWebhook(body)
	if err != nil {
		log.Error().Err(err).Msg("не удалось распарсить ЮKassa webhook")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if event.Type == "payment.succeeded" {
		paymentID := event.Object.ID
		userVKID := int64(toInt(event.Object.Metadata["vk_id"]))
		tariffID := toInt(event.Object.Metadata["tariff_id"])

		log.Info().
			Str("payment_id", paymentID).
			Int64("user_vk_id", userVKID).
			Int("tariff_id", tariffID).
			Msg("оплата успешна")

		if err := flows.ProcessSuccessfulPayment(r.Context(), s.flowDeps, paymentID, userVKID, tariffID); err != nil {
			log.Error().Err(err).Msg("ошибка обработки успешного платежа")
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWavespeedWebhook(w http.ResponseWriter, r *http.Request) {
	// WaveSpeed webhook — альтернатива polling'у
	// При наличии webhook_secret верифицируем подпись
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var event struct {
		ID      string   `json:"id"`
		Status  string   `json:"status"`
		Outputs []string `json:"outputs"`
		Error   string   `json:"error"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		log.Error().Err(err).Msg("не удалось распарсить WaveSpeed webhook")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	log.Info().Str("task_id", event.ID).Str("status", event.Status).Msg("WaveSpeed webhook")

	if event.Status == "completed" && len(event.Outputs) > 0 {
		gen, err := s.flowDeps.GenRepo.GetByTaskID(r.Context(), event.ID)
		if err != nil || gen == nil {
			log.Warn().Str("task_id", event.ID).Msg("генерация не найдена по task_id")
			w.WriteHeader(http.StatusOK)
			return
		}
		outputURL := event.Outputs[0]
		_ = s.flowDeps.GenRepo.SetCompleted(r.Context(), gen.ID, outputURL)
		_ = s.flowDeps.Sender.SendPhotoResult(r.Context(), gen.UserVKID, outputURL, gen.Model, "", "")
		wsSt, err2 := s.flowDeps.State.Get(r.Context(), gen.UserVKID)
		if err2 != nil || wsSt == nil {
			wsSt = &flows.State{}
		}
		wsSt.Step = flows.StepAfterGen
		wsSt.PhotoURL = outputURL
		_ = s.flowDeps.State.Set(r.Context(), gen.UserVKID, wsSt)
	}

	w.WriteHeader(http.StatusOK)
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	case int:
		return n
	}
	return 0
}

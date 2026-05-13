package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	s.router.Get("/vk/return", s.handleVKReturn)
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
		log.Error().Err(err).Msg("failed to parse vk event")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if event.Type == "confirmation" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, s.cfg.VKConfirmationToken)
		return
	}

	if s.cfg.VKSecret != "" && event.Secret != s.cfg.VKSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	go s.handler.Handle(context.WithoutCancel(r.Context()), &event)

	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, "ok")
}

func (s *Server) handleVKReturn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Оплата принята</title>
</head>
<body style="font-family:Arial,sans-serif;background:#f6f8fb;color:#162033;margin:0;padding:32px;">
  <main style="max-width:560px;margin:0 auto;background:#fff;border-radius:18px;padding:32px;box-shadow:0 10px 30px rgba(22,32,51,0.08);">
    <h1 style="margin-top:0;">Оплата принята</h1>
    <p style="line-height:1.6;">Платеж обработан. Вернитесь в сообщения VK, чтобы продолжить работу с ботом.</p>
  </main>
</body>
</html>`)
}

func (s *Server) handleYukassaWebhook(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("remote_addr", r.RemoteAddr).Str("path", r.URL.Path).Msg("yukassa webhook received")
	headers := sanitizeHeaders(r.Header)
	decision := "failed"
	payloadType := ""
	eventName := ""
	paymentID := ""
	paymentStatus := ""
	var metadata map[string]any
	var handledErr error

	defer func() {
		entry := log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("payload_type", payloadType).
			Str("event", eventName).
			Str("payment_id", paymentID).
			Str("payment_status", paymentStatus).
			Str("decision", decision).
			Any("metadata", metadata).
			Any("headers", headers)
		if handledErr != nil {
			entry = entry.Err(handledErr)
		}
		entry.Msg("handled yookassa webhook")
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handledErr = err
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Payment-Sha256-Signature")
	if err := s.yukassa.VerifyWebhookSignature(sig, body); err != nil {
		handledErr = err
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	event, err := s.yukassa.ParseWebhook(body)
	if err != nil {
		handledErr = err
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	payloadType = event.Type
	eventName = event.Event
	paymentID = event.Object.ID
	paymentStatus = event.Object.Status
	metadata = event.Object.Metadata

	if eventName == "" {
		decision = "ignored"
		w.WriteHeader(http.StatusOK)
		return
	}
	if paymentID == "" {
		handledErr = fmt.Errorf("yookassa webhook event %s has empty payment id", eventName)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	switch eventName {
	case "payment.succeeded", "payment.canceled":
	default:
		decision = "ignored"
		w.WriteHeader(http.StatusOK)
		return
	}

	paymentDetails, err := s.yukassa.GetPayment(r.Context(), paymentID)
	if err != nil {
		handledErr = err
		http.Error(w, "failed to verify payment", http.StatusInternalServerError)
		return
	}
	if len(paymentDetails.Metadata) > 0 {
		metadata = paymentDetails.Metadata
	}
	paymentStatus = paymentDetails.Status

	switch eventName {
	case "payment.succeeded":
		if paymentDetails.Status != "succeeded" {
			handledErr = fmt.Errorf("payment %s status mismatch: expected succeeded, got %s", paymentID, paymentDetails.Status)
			http.Error(w, "payment status mismatch", http.StatusInternalServerError)
			return
		}
		if err := flows.ProcessSuccessfulPaymentWithMetadata(r.Context(), s.flowDeps, paymentID, metadata); err != nil {
			handledErr = err
			http.Error(w, "failed to process payment", http.StatusInternalServerError)
			return
		}
	case "payment.canceled":
		if paymentDetails.Status != "canceled" {
			handledErr = fmt.Errorf("payment %s status mismatch: expected canceled, got %s", paymentID, paymentDetails.Status)
			http.Error(w, "payment status mismatch", http.StatusInternalServerError)
			return
		}
		if err := flows.ProcessCanceledPayment(r.Context(), s.flowDeps, paymentID); err != nil {
			handledErr = err
			http.Error(w, "failed to process payment", http.StatusInternalServerError)
			return
		}
	}

	decision = "processed"
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWavespeedWebhook(w http.ResponseWriter, r *http.Request) {
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
		log.Error().Err(err).Msg("failed to parse wavespeed webhook")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	log.Info().Str("task_id", event.ID).Str("status", event.Status).Msg("wavespeed webhook")

	if event.Status == "completed" && len(event.Outputs) > 0 {
		gen, err := s.flowDeps.GenRepo.GetByTaskID(r.Context(), event.ID)
		if err != nil || gen == nil {
			log.Warn().Str("task_id", event.ID).Msg("generation not found by task_id")
			w.WriteHeader(http.StatusOK)
			return
		}
		outputURL := event.Outputs[0]
		_ = s.flowDeps.GenRepo.SetCompleted(r.Context(), gen.ID, outputURL)
		if err := s.flowDeps.Sender.SendPhotoResult(r.Context(), gen.UserVKID, outputURL, gen.Model, "", ""); err != nil {
			log.Error().Err(err).Int64("generation_id", gen.ID).Msg("failed to send webhook generation result")
			_ = s.flowDeps.GenRepo.RefundGenerationCharge(r.Context(), gen.ID)
			_ = s.flowDeps.Sender.SendScreenText(r.Context(), gen.UserVKID, "worker_vk_upload_error", nil)
			w.WriteHeader(http.StatusOK)
			return
		}
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

func sanitizeHeaders(headers http.Header) map[string][]string {
	if len(headers) == 0 {
		return nil
	}

	out := make(map[string][]string, len(headers))
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "authorization") ||
			strings.Contains(lowerKey, "cookie") ||
			strings.Contains(lowerKey, "signature") ||
			strings.Contains(lowerKey, "secret") {
			out[key] = []string{"[redacted]"}
			continue
		}

		copied := make([]string, len(values))
		copy(copied, values)
		out[key] = copied
	}
	return out
}

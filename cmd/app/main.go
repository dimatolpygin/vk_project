package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"vk_neuro_bot/internal/admin"
	"vk_neuro_bot/internal/bot"
	"vk_neuro_bot/internal/bot/flows"
	"vk_neuro_bot/internal/config"
	"vk_neuro_bot/internal/db"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/s3"
	"vk_neuro_bot/internal/vkgroup"
	"vk_neuro_bot/internal/wavespeed"
	"vk_neuro_bot/internal/worker"
	"vk_neuro_bot/internal/yukassa"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "02.01.2006 15:04:05"})

	cfg := config.Load()
	if cfg.LogLevel == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// PostgreSQL
	pool := db.New(ctx, cfg.DBURL)
	defer pool.Close()

	runMigrations(cfg.DBURL)

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("Redis не отвечает на ping")
	}
	log.Info().Msg("Redis подключён")
	defer func() { _ = rdb.Close() }()

	// Repositories
	userRepo := repository.NewUserRepo(pool)
	genRepo := repository.NewGenerationRepo(pool)
	tariffRepo := repository.NewTariffRepo(pool)
	orderRepo := repository.NewOrderRepo(pool)
	msgRepo := repository.NewMessageRepo(pool)
	broadcastRepo := repository.NewBroadcastRepo(pool)
	catRepo := repository.NewCategoryRepo(pool)
	promptRepo := repository.NewPromptRepo(pool)
	refRepo := repository.NewReferralRepo(pool)
	statsRepo := repository.NewStatsRepo(pool)
	activityRepo := repository.NewActivityRepo(pool)
	if err := msgRepo.EnsureDefaults(ctx); err != nil {
		log.Fatal().Err(err).Msg("не удалось синхронизировать экраны контента")
	}

	// External clients
	vkClient := vkgroup.New(cfg.VKGroupToken, cfg.VKGroupID)
	wsClient := wavespeed.New(cfg.WavespeedAPIKey)
	ykClient := yukassa.New(
		cfg.YukassaShopID,
		cfg.YukassaSecretKey,
		cfg.YukassaWebhookSecret,
		cfg.YukassaReceiptEmail,
		cfg.YukassaReceiptVATCode,
		cfg.YukassaReceiptPaymentSubject,
		cfg.YukassaReceiptPaymentMode,
	)

	var s3Client *s3.Client
	if sc, err := s3.New(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region); err != nil {
		log.Warn().Err(err).Msg("S3 не настроен, продолжаем без хранилища")
	} else {
		s3Client = sc
	}

	var photoStorage flows.PhotoStorage
	if s3Client != nil {
		photoStorage = s3Client
	}
	var adminStorage admin.Storage
	if s3Client != nil {
		adminStorage = s3Client
	}

	// Asynq
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer func() { _ = asynqClient.Close() }()

	// Bot
	stateMgr := bot.NewStateManager(rdb)
	sender := bot.NewSender(vkClient, msgRepo, userRepo, broadcastRepo, stateMgr)

	deps := &flows.Deps{
		Sender:        sender,
		State:         stateMgr,
		UserRepo:      userRepo,
		GenRepo:       genRepo,
		TariffRepo:    tariffRepo,
		OrderRepo:     orderRepo,
		MsgRepo:       msgRepo,
		CatRepo:       catRepo,
		PromptRepo:    promptRepo,
		RefRepo:       refRepo,
		StatsRepo:     statsRepo,
		ActivityRepo:  activityRepo,
		AsynqClient:   asynqClient,
		WaveSpeed:     wsClient,
		Yukassa:       ykClient,
		VKClient:      vkClient,
		VKGroupID:     cfg.VKGroupID,
		VKGroupURL:    fmt.Sprintf("https://vk.com/club%d", cfg.VKGroupID),
		DefaultModel:  cfg.WavespeedModel,
		PublicBaseURL: cfg.PublicBaseURL,
		Storage:       photoStorage,
	}

	registry := flows.NewRegistry(deps)
	handler := bot.NewHandler(stateMgr, sender, userRepo, statsRepo, activityRepo, registry, vkClient)
	botServer := bot.NewServer(cfg, handler, ykClient, deps)

	adminServer := admin.NewServer(
		cfg.AdminLogin, cfg.AdminPassword,
		userRepo, tariffRepo, msgRepo, broadcastRepo, catRepo, promptRepo, statsRepo, orderRepo, rdb,
		asynqClient,
		adminStorage,
	)

	botHTTP := &http.Server{
		Addr:         ":" + cfg.BotPort,
		Handler:      botServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	adminHTTP := &http.Server{
		Addr:         ":" + cfg.AdminPort,
		Handler:      adminServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Worker (asynq)
	asynqSrv := worker.NewAsynqServer(cfg.RedisAddr, cfg.RedisPassword)
	mux := asynq.NewServeMux()
	var workerStorage worker.PhotoStorage
	if s3Client != nil {
		workerStorage = s3Client
	}
	generateHandler := worker.NewGenerateHandler(genRepo, sender, wsClient, workerStorage, rdb)
	broadcastHandler := worker.NewBroadcastHandler(broadcastRepo, sender, asynqClient)
	mux.HandleFunc(worker.TaskGenerate, generateHandler.ProcessTask)
	mux.HandleFunc(worker.TaskBroadcastProcess, broadcastHandler.ProcessTask)

	go func() {
		log.Info().Str("addr", botHTTP.Addr).Msg("бот запущен")
		if err := botHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("ошибка бот-сервера")
		}
	}()

	go func() {
		log.Info().Str("addr", adminHTTP.Addr).Msg("админка запущена")
		if err := adminHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("ошибка admin-сервера")
		}
	}()

	go func() {
		log.Info().Msg("asynq worker запущен")
		if err := asynqSrv.Run(mux); err != nil {
			log.Fatal().Err(err).Msg("ошибка asynq worker")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("завершение работы...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = botHTTP.Shutdown(shutCtx)
	_ = adminHTTP.Shutdown(shutCtx)
	asynqSrv.Shutdown()
}

func runMigrations(dsn string) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("не удалось открыть соединение для миграций")
	}
	defer func() { _ = sqlDB.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal().Err(err).Msg("ошибка установки диалекта goose")
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		log.Fatal().Err(err).Msg("ошибка выполнения миграций")
	}
	log.Info().Msg("миграции применены")
}

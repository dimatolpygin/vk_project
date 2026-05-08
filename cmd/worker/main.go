package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"vk_neuro_bot/internal/bot"
	"vk_neuro_bot/internal/config"
	"vk_neuro_bot/internal/db"
	"vk_neuro_bot/internal/repository"
	"vk_neuro_bot/internal/s3"
	"vk_neuro_bot/internal/vkgroup"
	"vk_neuro_bot/internal/wavespeed"
	"vk_neuro_bot/internal/worker"
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

	pool := db.New(ctx, cfg.DBURL)
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("Redis не отвечает на ping")
	}
	defer func() { _ = rdb.Close() }()

	genRepo := repository.NewGenerationRepo(pool)
	msgRepo := repository.NewMessageRepo(pool)
	userRepo := repository.NewUserRepo(pool)
	if err := msgRepo.WaitForKeyboardSchema(ctx, 30*time.Second); err != nil {
		log.Fatal().Err(err).Msg("не удалось дождаться миграции content-экранов")
	}
	if err := msgRepo.EnsureDefaults(ctx); err != nil {
		log.Fatal().Err(err).Msg("не удалось синхронизировать экраны контента")
	}
	vkClient := vkgroup.New(cfg.VKGroupToken, cfg.VKGroupID)
	wsClient := wavespeed.New(cfg.WavespeedAPIKey)
	stateMgr := bot.NewStateManager(rdb)
	sender := bot.NewSender(vkClient, msgRepo, userRepo, stateMgr)

	var s3Client *s3.Client
	if sc, err := s3.New(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region); err != nil {
		log.Warn().Err(err).Msg("S3 не настроен, продолжаем без хранилища")
	} else {
		s3Client = sc
	}
	var workerStorage worker.PhotoStorage
	if s3Client != nil {
		workerStorage = s3Client
	}

	generateHandler := worker.NewGenerateHandler(genRepo, userRepo, sender, wsClient, workerStorage, rdb)

	srv := worker.NewAsynqServer(cfg.RedisAddr, cfg.RedisPassword)
	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskGenerate, generateHandler.ProcessTask)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Msg("asynq worker запущен")
		if err := srv.Run(mux); err != nil {
			log.Fatal().Err(err).Msg("ошибка запуска asynq worker")
		}
	}()

	<-quit
	log.Info().Msg("завершение воркера...")
	srv.Shutdown()
}

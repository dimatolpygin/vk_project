package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"vk_neuro_bot/internal/bot"
	"vk_neuro_bot/internal/config"
	"vk_neuro_bot/internal/db"
	"vk_neuro_bot/internal/repository"
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
	defer rdb.Close()

	genRepo := repository.NewGenerationRepo(pool)
	msgRepo := repository.NewMessageRepo(pool)
	vkClient := vkgroup.New(cfg.VKGroupToken, cfg.VKGroupID)
	wsClient := wavespeed.New(cfg.WavespeedAPIKey)
	sender := bot.NewSender(vkClient, msgRepo)

	generateHandler := worker.NewGenerateHandler(genRepo, sender, wsClient)

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

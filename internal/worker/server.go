package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

func NewAsynqServer(redisAddr, redisPassword string) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
		},
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"default":  10,
				"critical": 20,
				// Видео занимает воркер минутами. Своя очередь с низким весом
				// оставляет фото-задачам дорогу, даже когда видео стоит очередь.
				"video": 5,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().Str("task", task.Type()).Err(err).Msg("ошибка обработки задачи asynq")
			}),
		},
	)
}

func NewAsynqClient(redisAddr, redisPassword string) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
	})
}

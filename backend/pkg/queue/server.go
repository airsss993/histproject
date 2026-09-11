package queue

import (
	"github.com/airsss993/histproject-backend/internal/config"
	"github.com/hibiken/asynq"
)

type Server struct {
	*asynq.Server
}

// NewServer - функция для создания нового экземпляра Server с конфигурацией Redis
func NewServer(cfg config.Redis) *Server {
	return &Server{
		asynq.NewServer(asynq.RedisClientOpt{
			Addr:     cfg.RedisAddress,
			DB:       cfg.RedisDb,
			Password: cfg.RedisPassword,
		}, asynq.Config{
			Concurrency: 1,
		}),
	}
}

package queue

import (
	"github.com/airsss993/histproject-backend/internal/config"
	"github.com/hibiken/asynq"
)

var QueuClient *Client

type Client struct {
	*asynq.Client
}

// NewClient - функция для создания нового экземпляра Client с конфигурацией Redis
func NewClient(cfg config.Redis) *Client {
	return &Client{
		asynq.NewClient(asynq.RedisClientOpt{
			Addr:     cfg.RedisAddress,
			DB:       cfg.RedisDb,
			Password: cfg.RedisPassword,
		}),
	}
}

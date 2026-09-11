package worker

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

// Типы задач для асинхронной обработки
const (
	TypeProcessArchive = "archive:process"
)

// ProcessArchivePayload - структура данных для задачи обработки архива
type ProcessArchivePayload struct {
	RequestId int
	ArchiveId string
}

// NewProcessArchiveTask - функция для создания новой задачи обработки архива
func NewProcessArchiveTask(requestId int, archiveId string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProcessArchivePayload{
		RequestId: requestId,
		ArchiveId: archiveId,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProcessArchive, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(15*time.Minute),
	), nil
}

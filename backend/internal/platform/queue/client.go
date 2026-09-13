package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	ArchiveProcessTask         = "archive:process"
	SubmissionNotificationTask = "notification:submission"
)

type ArchiveProcessPayload struct {
	RequestID int64  `json:"RequestId"`
	ArchiveID string `json:"ArchiveId"`
}

type SubmissionNotificationPayload struct {
	SubmissionID int64  `json:"submissionId"`
	Action       string `json:"action"`
}

type enqueuer interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

type Client struct {
	enqueuer enqueuer
}

func NewClient(redisClient redis.UniversalClient) *Client {
	return &Client{enqueuer: asynq.NewClientFromRedisClient(redisClient)}
}

func (client *Client) EnqueueArchive(ctx context.Context, submissionID int64, archiveID string) error {
	payload, err := json.Marshal(ArchiveProcessPayload{RequestID: submissionID, ArchiveID: archiveID})
	if err != nil {
		return fmt.Errorf("encode archive task: %w", err)
	}
	task := asynq.NewTask(ArchiveProcessTask, payload)
	_, err = client.enqueuer.EnqueueContext(ctx, task,
		asynq.TaskID(fmt.Sprintf("submission-%d-archive", submissionID)),
		asynq.MaxRetry(3),
		asynq.Timeout(15*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("enqueue archive task: %w", err)
	}
	return nil
}

func (client *Client) EnqueueSubmissionNotification(ctx context.Context, submissionID int64, action string) error {
	payload, err := json.Marshal(SubmissionNotificationPayload{SubmissionID: submissionID, Action: action})
	if err != nil {
		return fmt.Errorf("encode submission notification task: %w", err)
	}
	task := asynq.NewTask(SubmissionNotificationTask, payload)
	_, err = client.enqueuer.EnqueueContext(ctx, task,
		asynq.TaskID(fmt.Sprintf("submission-%d-notification-%s", submissionID, action)),
		asynq.MaxRetry(10),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("enqueue submission notification task: %w", err)
	}
	return nil
}

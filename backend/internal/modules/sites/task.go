package sites

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/hibiken/asynq"
)

var ErrInvalidTaskPayload = errors.New("invalid archive task payload")

type TaskProcessor interface {
	Process(context.Context, int64, string) error
}

func NewTaskMux(processor TaskProcessor) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(platformqueue.ArchiveProcessTask, func(ctx context.Context, task *asynq.Task) error {
		var payload platformqueue.ArchiveProcessPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return errors.Join(fmt.Errorf("%w: %v", ErrInvalidTaskPayload, err), asynq.SkipRetry)
		}
		if payload.RequestID <= 0 || strings.TrimSpace(payload.ArchiveID) == "" {
			return errors.Join(ErrInvalidTaskPayload, asynq.SkipRetry)
		}
		if err := processor.Process(ctx, payload.RequestID, payload.ArchiveID); err != nil {
			if errors.Is(err, ErrArchiveMismatch) || errors.Is(err, ErrUnsafeSlug) {
				return errors.Join(fmt.Errorf("process archive task: %w", err), asynq.SkipRetry)
			}
			return fmt.Errorf("process archive task: %w", err)
		}
		return nil
	})
	return mux
}

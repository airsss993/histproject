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
	Fail(context.Context, int64) error
}

func NewTaskMux(processor TaskProcessor) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(platformqueue.ArchiveProcessTask, func(ctx context.Context, task *asynq.Task) error {
		var payload platformqueue.ArchiveProcessPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return errors.Join(fmt.Errorf("%w: %v", ErrInvalidTaskPayload, err), asynq.SkipRetry)
		}
		if payload.SubmissionID <= 0 || strings.TrimSpace(payload.ArchiveID) == "" {
			return errors.Join(ErrInvalidTaskPayload, asynq.SkipRetry)
		}
		if err := processor.Process(ctx, payload.SubmissionID, payload.ArchiveID); err != nil {
			err = fmt.Errorf("process archive task: %w", err)
			retry, retryKnown := asynq.GetRetryCount(ctx)
			maxRetry, maxRetryKnown := asynq.GetMaxRetry(ctx)
			// A task that will not run again must not leave the submission in processing, where no admin action reaches it.
			// ErrArchiveMismatch is excluded: the task is not about this submission's archive.
			if errors.Is(err, ErrUnsafeSlug) || (retryKnown && maxRetryKnown && retry >= maxRetry && !errors.Is(err, ErrArchiveMismatch)) {
				err = errors.Join(err, processor.Fail(ctx, payload.SubmissionID))
			}
			if errors.Is(err, ErrArchiveMismatch) || errors.Is(err, ErrUnsafeSlug) {
				return errors.Join(err, asynq.SkipRetry)
			}
			return err
		}
		return nil
	})
	return mux
}

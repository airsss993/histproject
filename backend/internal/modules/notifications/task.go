package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/hibiken/asynq"
)

var ErrInvalidTaskPayload = errors.New("invalid notification task payload")

type Deliverer interface {
	Deliver(context.Context, int64, Action) error
}

func RegisterTasks(mux *asynq.ServeMux, deliverer Deliverer) {
	mux.HandleFunc(platformqueue.SubmissionNotificationTask, func(ctx context.Context, task *asynq.Task) error {
		var payload platformqueue.SubmissionNotificationPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return errors.Join(fmt.Errorf("%w: %v", ErrInvalidTaskPayload, err), asynq.SkipRetry)
		}
		action := Action(payload.Action)
		if payload.SubmissionID <= 0 || !action.Valid() {
			return errors.Join(ErrInvalidTaskPayload, asynq.SkipRetry)
		}
		if err := deliverer.Deliver(ctx, payload.SubmissionID, action); err != nil {
			return fmt.Errorf("deliver submission notification: %w", err)
		}
		return nil
	})
}

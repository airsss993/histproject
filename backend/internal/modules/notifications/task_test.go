package notifications

import (
	"context"
	"errors"
	"testing"

	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/hibiken/asynq"
)

func TestTaskHandlerDeliversSubmissionNotification(t *testing.T) {
	deliverer := &delivererStub{}
	mux := asynq.NewServeMux()
	RegisterTasks(mux, deliverer)
	task := asynq.NewTask(platformqueue.SubmissionNotificationTask, []byte(`{"submissionId":7,"action":"reviewed"}`))

	if err := mux.ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if deliverer.ctx != t.Context() || deliverer.submissionID != 7 || deliverer.action != ActionReviewed {
		t.Fatalf("delivery = context:%v submission:%d action:%q", deliverer.ctx == t.Context(), deliverer.submissionID, deliverer.action)
	}
}

func TestTaskHandlerSkipsInvalidNotificationTask(t *testing.T) {
	tests := []string{
		`{`,
		`{"submissionId":0,"action":"reviewed"}`,
		`{"submissionId":7,"action":"unknown"}`,
	}
	for _, payload := range tests {
		deliverer := &delivererStub{}
		mux := asynq.NewServeMux()
		RegisterTasks(mux, deliverer)

		err := mux.ProcessTask(t.Context(), asynq.NewTask(platformqueue.SubmissionNotificationTask, []byte(payload)))
		if !errors.Is(err, ErrInvalidTaskPayload) || !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("ProcessTask() error = %v", err)
		}
		if deliverer.called {
			t.Fatal("deliverer called for invalid payload")
		}
	}
}

type delivererStub struct {
	ctx          context.Context
	submissionID int64
	action       Action
	called       bool
	err          error
}

func (stub *delivererStub) Deliver(ctx context.Context, submissionID int64, action Action) error {
	stub.ctx = ctx
	stub.submissionID = submissionID
	stub.action = action
	stub.called = true
	return stub.err
}

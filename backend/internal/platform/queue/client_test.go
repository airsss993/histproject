package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type enqueuerStub struct {
	ctx     context.Context
	task    *asynq.Task
	options []asynq.Option
}

func (stub *enqueuerStub) EnqueueContext(ctx context.Context, task *asynq.Task, options ...asynq.Option) (*asynq.TaskInfo, error) {
	stub.ctx = ctx
	stub.task = task
	stub.options = options
	return &asynq.TaskInfo{}, nil
}

func TestEnqueueArchiveUsesContextCompatiblePayloadAndUniqueTaskID(t *testing.T) {
	enqueuer := &enqueuerStub{}
	client := &Client{enqueuer: enqueuer}
	ctx := t.Context()

	if err := client.EnqueueArchive(ctx, 42, "archive-id"); err != nil {
		t.Fatalf("EnqueueArchive() error = %v", err)
	}
	if enqueuer.ctx != ctx {
		t.Fatal("EnqueueArchive() did not forward context")
	}
	if enqueuer.task.Type() != ArchiveProcessTask {
		t.Fatalf("task type = %q, want %q", enqueuer.task.Type(), ArchiveProcessTask)
	}
	var payload ArchiveProcessPayload
	if err := json.Unmarshal(enqueuer.task.Payload(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.RequestID != 42 || payload.ArchiveID != "archive-id" {
		t.Fatalf("payload = %#v", payload)
	}

	wantOptions := map[asynq.OptionType]any{
		asynq.TaskIDOpt:   "submission-42-archive",
		asynq.MaxRetryOpt: 3,
		asynq.TimeoutOpt:  15 * time.Minute,
	}
	for _, option := range enqueuer.options {
		if want, ok := wantOptions[option.Type()]; ok {
			if option.Value() != want {
				t.Errorf("option %v = %v, want %v", option.Type(), option.Value(), want)
			}
			delete(wantOptions, option.Type())
		}
	}
	if len(wantOptions) != 0 {
		t.Fatalf("missing options = %#v", wantOptions)
	}
}

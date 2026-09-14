package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type enqueuerStub struct {
	ctx     context.Context
	task    *asynq.Task
	options []asynq.Option
	err     error
}

func (stub *enqueuerStub) EnqueueContext(ctx context.Context, task *asynq.Task, options ...asynq.Option) (*asynq.TaskInfo, error) {
	stub.ctx = ctx
	stub.task = task
	stub.options = options
	return &asynq.TaskInfo{}, stub.err
}

func TestEnqueueSubmissionNotificationTreatsTaskIDConflictAsSuccess(t *testing.T) {
	client := &Client{enqueuer: &enqueuerStub{err: asynq.ErrTaskIDConflict}}

	if err := client.EnqueueSubmissionNotification(t.Context(), 42, "published"); err != nil {
		t.Fatalf("EnqueueSubmissionNotification() error = %v", err)
	}
}

func TestEnqueueSubmissionNotificationReturnsRedisError(t *testing.T) {
	want := errors.New("Redis unavailable")
	client := &Client{enqueuer: &enqueuerStub{err: want}}

	err := client.EnqueueSubmissionNotification(t.Context(), 42, "published")
	if !errors.Is(err, want) {
		t.Fatalf("EnqueueSubmissionNotification() error = %v, want Redis error", err)
	}
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
	if payload.SubmissionID != 42 || payload.ArchiveID != "archive-id" {
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

func TestEnqueueSubmissionNotificationUsesMinimalPayloadAndUniqueTaskID(t *testing.T) {
	enqueuer := &enqueuerStub{}
	client := &Client{enqueuer: enqueuer}
	ctx := t.Context()

	if err := client.EnqueueSubmissionNotification(ctx, 42, "reviewed"); err != nil {
		t.Fatalf("EnqueueSubmissionNotification() error = %v", err)
	}
	if enqueuer.ctx != ctx || enqueuer.task.Type() != SubmissionNotificationTask {
		t.Fatalf("task = context:%v type:%q", enqueuer.ctx == ctx, enqueuer.task.Type())
	}
	var payload SubmissionNotificationPayload
	if err := json.Unmarshal(enqueuer.task.Payload(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SubmissionID != 42 || payload.Action != "reviewed" {
		t.Fatalf("payload = %#v", payload)
	}
	if string(enqueuer.task.Payload()) != `{"submissionId":42,"action":"reviewed"}` {
		t.Fatalf("raw payload = %s", enqueuer.task.Payload())
	}

	wantOptions := map[asynq.OptionType]any{
		asynq.TaskIDOpt:   "submission-42-notification-reviewed",
		asynq.MaxRetryOpt: 10,
		asynq.TimeoutOpt:  30 * time.Second,
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

package sites

import (
	"context"
	"errors"
	"testing"

	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/hibiken/asynq"
)

func TestArchiveTaskHandlerProcessesLegacyPayload(t *testing.T) {
	processor := &taskProcessorStub{}
	mux := NewTaskMux(processor)
	task := asynq.NewTask(platformqueue.ArchiveProcessTask, []byte(`{"RequestId":7,"ArchiveId":"archive-id"}`))

	if err := mux.ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if processor.ctx != t.Context() || processor.submissionID != 7 || processor.archiveID != "archive-id" {
		t.Fatalf("processor call = context:%v submission:%d archive:%q", processor.ctx == t.Context(), processor.submissionID, processor.archiveID)
	}
}

func TestArchiveTaskHandlerRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "JSON", payload: `{`},
		{name: "submission ID", payload: `{"RequestId":0,"ArchiveId":"archive-id"}`},
		{name: "archive ID", payload: `{"RequestId":7,"ArchiveId":""}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processor := &taskProcessorStub{}
			task := asynq.NewTask(platformqueue.ArchiveProcessTask, []byte(test.payload))

			err := NewTaskMux(processor).ProcessTask(t.Context(), task)
			if !errors.Is(err, ErrInvalidTaskPayload) {
				t.Fatalf("ProcessTask() error = %v, want ErrInvalidTaskPayload", err)
			}
			if processor.called {
				t.Fatal("processor called for invalid payload")
			}
		})
	}
}

func TestArchiveTaskHandlerReturnsProcessorError(t *testing.T) {
	want := errors.New("processing failed")
	processor := &taskProcessorStub{err: want}
	task := asynq.NewTask(platformqueue.ArchiveProcessTask, []byte(`{"RequestId":7,"ArchiveId":"archive-id"}`))

	err := NewTaskMux(processor).ProcessTask(t.Context(), task)
	if !errors.Is(err, want) {
		t.Fatalf("ProcessTask() error = %v, want processor error", err)
	}
}

type taskProcessorStub struct {
	ctx          context.Context
	submissionID int64
	archiveID    string
	called       bool
	err          error
}

func (stub *taskProcessorStub) Process(ctx context.Context, submissionID int64, archiveID string) error {
	stub.ctx = ctx
	stub.submissionID = submissionID
	stub.archiveID = archiveID
	stub.called = true
	return stub.err
}

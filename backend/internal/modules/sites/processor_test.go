package sites

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/airsss993/histproject/backend/internal/modules/submissions"
)

func TestProcessorPublishesValidatedSite(t *testing.T) {
	data := makeZIP(t,
		zipEntry{name: "index.html", body: "<h1>site</h1>"},
		zipEntry{name: "assets/app.js", body: "console.log('ok')"},
	)
	archive := newMemoryArchive(data)
	submissionService := &submissionServiceStub{submission: processingSubmission()}
	storage := &siteStorageStub{}
	screenshotter := &screenshotterStub{image: []byte("png")}
	processor := NewProcessor(submissionService, func(ctx context.Context, key string) (io.ReaderAt, io.Closer, int64, error) {
		if ctx != t.Context() || key != "archive-id" {
			t.Fatalf("load archive = context:%v key:%q", ctx == t.Context(), key)
		}
		return archive, archive, int64(len(data)), nil
	}, storage, screenshotter, processorConfig())

	if err := processor.Process(t.Context(), 7, "archive-id"); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !archive.closed {
		t.Fatal("archive was not closed")
	}
	if screenshotter.siteURL != "http://minio:9000/sites/my-event/index.html" {
		t.Fatalf("screenshot URL = %q", screenshotter.siteURL)
	}
	if len(storage.uploads) != 3 {
		t.Fatalf("uploads = %#v", storage.uploads)
	}
	if storage.uploads[0].key != "my-event/index.html" || storage.uploads[0].body != "<h1>site</h1>" {
		t.Fatalf("index upload = %#v", storage.uploads[0])
	}
	if storage.uploads[1].key != "my-event/assets/app.js" || storage.uploads[1].body != "console.log('ok')" {
		t.Fatalf("asset upload = %#v", storage.uploads[1])
	}
	if storage.uploads[2].key != "my-event/screenshot.png" || storage.uploads[2].body != "png" || storage.uploads[2].contentType != "image/png" {
		t.Fatalf("screenshot upload = %#v", storage.uploads[2])
	}
	if submissionService.readySiteURL != "https://example.test/preview/my-event/index.html" {
		t.Fatalf("ready site URL = %q", submissionService.readySiteURL)
	}
	if submissionService.readyScreenshotURL != "https://cdn.example.test/sites/my-event/screenshot.png" {
		t.Fatalf("ready screenshot URL = %q", submissionService.readyScreenshotURL)
	}
}

func TestProcessorRejectsInvalidArchiveWithoutRetry(t *testing.T) {
	data := makeZIP(t, zipEntry{name: "nested/index.html", body: "site"})
	archive := newMemoryArchive(data)
	submissionService := &submissionServiceStub{submission: processingSubmission()}
	storage := &siteStorageStub{}
	screenshotter := &screenshotterStub{}
	processor := NewProcessor(submissionService, archiveLoader(data, archive), storage, screenshotter, processorConfig())

	if err := processor.Process(t.Context(), 7, "archive-id"); err != nil {
		t.Fatalf("Process() error = %v, want permanent rejection", err)
	}
	if submissionService.rejectedReason != "В корне архива отсутствует index.html" {
		t.Fatalf("rejected reason = %q", submissionService.rejectedReason)
	}
	if len(storage.uploads) != 0 || screenshotter.calls != 0 {
		t.Fatalf("uploads = %d, screenshots = %d", len(storage.uploads), screenshotter.calls)
	}
}

func TestProcessorRejectsArchiveWithInvalidChecksum(t *testing.T) {
	data := makeZIP(t, zipEntry{name: "index.html", body: "site", store: true})
	contentOffset := bytes.Index(data, []byte("site"))
	if contentOffset < 0 {
		t.Fatal("stored ZIP content not found")
	}
	data[contentOffset] = 'S'
	archive := newMemoryArchive(data)
	submissionService := &submissionServiceStub{submission: processingSubmission()}
	storage := &siteStorageStub{}
	processor := NewProcessor(
		submissionService,
		archiveLoader(data, archive),
		storage,
		&screenshotterStub{},
		processorConfig(),
	)

	if err := processor.Process(t.Context(), 7, "archive-id"); err != nil {
		t.Fatalf("Process() error = %v, want permanent rejection", err)
	}
	if submissionService.rejectedReason != "Архив не прошел проверку безопасности" {
		t.Fatalf("rejected reason = %q", submissionService.rejectedReason)
	}
	if len(storage.uploads) != 0 {
		t.Fatalf("uploads = %#v", storage.uploads)
	}
}

func TestProcessorCleansUploadedFilesAfterTransientFailure(t *testing.T) {
	data := makeZIP(t,
		zipEntry{name: "index.html", body: "site"},
		zipEntry{name: "app.js", body: "code"},
	)
	archive := newMemoryArchive(data)
	storage := &siteStorageStub{putErrorKey: "my-event/app.js", putError: errors.New("MinIO unavailable")}
	processor := NewProcessor(
		&submissionServiceStub{submission: processingSubmission()},
		archiveLoader(data, archive),
		storage,
		&screenshotterStub{},
		processorConfig(),
	)

	err := processor.Process(t.Context(), 7, "archive-id")
	if err == nil || !strings.Contains(err.Error(), "upload site file") {
		t.Fatalf("Process() error = %v", err)
	}
	if len(storage.deleted) != 2 || storage.deleted[0] != "my-event/app.js" || storage.deleted[1] != "my-event/index.html" {
		t.Fatalf("deleted keys = %#v", storage.deleted)
	}
}

func TestProcessorKeepsFilesWhenReadyTransitionIsUncertain(t *testing.T) {
	data := makeZIP(t, zipEntry{name: "index.html", body: "site"})
	archive := newMemoryArchive(data)
	storage := &siteStorageStub{}
	submissionService := &submissionServiceStub{
		submission: processingSubmission(),
		readyError: errors.New("database timeout"),
	}
	processor := NewProcessor(
		submissionService,
		archiveLoader(data, archive),
		storage,
		&screenshotterStub{image: []byte("png")},
		processorConfig(),
	)

	err := processor.Process(t.Context(), 7, "archive-id")
	if err == nil || !strings.Contains(err.Error(), "mark submission ready") {
		t.Fatalf("Process() error = %v", err)
	}
	if len(storage.deleted) != 0 {
		t.Fatalf("deleted keys = %#v, want files preserved", storage.deleted)
	}
}

func TestProcessorSkipsCompletedSubmission(t *testing.T) {
	submission := processingSubmission()
	submission.Status = submissions.StatusNew
	submissionService := &submissionServiceStub{submission: submission}
	loaded := false
	processor := NewProcessor(submissionService, func(context.Context, string) (io.ReaderAt, io.Closer, int64, error) {
		loaded = true
		return nil, nil, 0, nil
	}, &siteStorageStub{}, &screenshotterStub{}, processorConfig())

	if err := processor.Process(t.Context(), 7, "archive-id"); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if loaded {
		t.Fatal("completed submission archive was loaded")
	}
}

func TestProcessorRejectsMismatchedArchiveID(t *testing.T) {
	processor := NewProcessor(
		&submissionServiceStub{submission: processingSubmission()},
		func(context.Context, string) (io.ReaderAt, io.Closer, int64, error) {
			t.Fatal("mismatched archive was loaded")
			return nil, nil, 0, nil
		},
		&siteStorageStub{},
		&screenshotterStub{},
		processorConfig(),
	)

	err := processor.Process(t.Context(), 7, "different-archive")
	if !errors.Is(err, ErrArchiveMismatch) {
		t.Fatalf("Process() error = %v, want ErrArchiveMismatch", err)
	}
}

func processingSubmission() submissions.Submission {
	return submissions.Submission{
		ID: 7, ArchiveID: "archive-id", Slug: "my-event", Status: submissions.StatusProcessing,
	}
}

func processorConfig() ProcessorConfig {
	return ProcessorConfig{
		Limits:            testLimits(),
		RenderBaseURL:     "http://minio:9000/sites",
		PreviewBaseURL:    "https://example.test/preview",
		PublicSiteBaseURL: "https://cdn.example.test/sites",
	}
}

type memoryArchive struct {
	*bytes.Reader
	closed bool
}

func newMemoryArchive(data []byte) *memoryArchive {
	return &memoryArchive{Reader: bytes.NewReader(data)}
}

func (archive *memoryArchive) Close() error {
	archive.closed = true
	return nil
}

func archiveLoader(data []byte, archive *memoryArchive) ArchiveLoader {
	return func(context.Context, string) (io.ReaderAt, io.Closer, int64, error) {
		return archive, archive, int64(len(data)), nil
	}
}

type submissionServiceStub struct {
	submission         submissions.Submission
	getError           error
	readySiteURL       string
	readyScreenshotURL string
	readyError         error
	rejectedReason     string
	rejectError        error
}

func (stub *submissionServiceStub) Get(context.Context, int64) (submissions.Submission, error) {
	return stub.submission, stub.getError
}

func (stub *submissionServiceStub) MarkReady(_ context.Context, _ int64, siteURL, screenshotURL string) (submissions.Submission, error) {
	stub.readySiteURL = siteURL
	stub.readyScreenshotURL = screenshotURL
	return stub.submission, stub.readyError
}

func (stub *submissionServiceStub) RejectProcessing(_ context.Context, _ int64, reason string) (submissions.Submission, error) {
	stub.rejectedReason = reason
	return stub.submission, stub.rejectError
}

type siteUpload struct {
	key         string
	body        string
	contentType string
}

type siteStorageStub struct {
	uploads     []siteUpload
	deleted     []string
	putErrorKey string
	putError    error
}

func (stub *siteStorageStub) PutSiteFile(_ context.Context, key string, reader io.Reader, _ int64, contentType string) error {
	if key == stub.putErrorKey {
		return stub.putError
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	stub.uploads = append(stub.uploads, siteUpload{key: key, body: string(body), contentType: contentType})
	return nil
}

func (stub *siteStorageStub) DeleteSiteFile(_ context.Context, key string) error {
	stub.deleted = append(stub.deleted, key)
	return nil
}

type screenshotterStub struct {
	image   []byte
	err     error
	siteURL string
	calls   int
}

func (stub *screenshotterStub) Capture(_ context.Context, siteURL string) ([]byte, error) {
	stub.calls++
	stub.siteURL = siteURL
	return stub.image, stub.err
}

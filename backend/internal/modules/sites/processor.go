package sites

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/submissions"
)

var (
	ErrArchiveMismatch = errors.New("archive does not belong to submission")
	ErrEmptyScreenshot = errors.New("screenshot is empty")
	ErrUnsafeSlug      = errors.New("submission slug is unsafe")
)

type ArchiveLoader func(context.Context, string) (io.ReaderAt, io.Closer, int64, error)

type SubmissionService interface {
	Get(context.Context, int64) (submissions.Submission, error)
	MarkReady(context.Context, int64, string, string) (submissions.Submission, error)
	RejectProcessing(context.Context, int64, string) (submissions.Submission, error)
}

type SiteStorage interface {
	PutSiteFile(context.Context, string, io.Reader, int64, string) error
	DeleteSiteFile(context.Context, string) error
}

type Screenshotter interface {
	Capture(context.Context, string) ([]byte, error)
}

type ProcessorConfig struct {
	Limits            Limits
	RenderBaseURL     string
	PreviewBaseURL    string
	PublicSiteBaseURL string
}

type Processor struct {
	submissions   SubmissionService
	loadArchive   ArchiveLoader
	storage       SiteStorage
	screenshotter Screenshotter
	config        ProcessorConfig
}

func NewProcessor(submissionService SubmissionService, loadArchive ArchiveLoader, storage SiteStorage, screenshotter Screenshotter, cfg ProcessorConfig) *Processor {
	return &Processor{
		submissions: submissionService, loadArchive: loadArchive, storage: storage,
		screenshotter: screenshotter, config: cfg,
	}
}

func (processor *Processor) Process(ctx context.Context, submissionID int64, archiveID string) (returnErr error) {
	submission, err := processor.submissions.Get(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}
	if submission.ArchiveID != archiveID {
		return ErrArchiveMismatch
	}
	if submission.Status != submissions.StatusProcessing {
		return nil
	}
	if !safeSlug(submission.Slug) {
		return ErrUnsafeSlug
	}

	archiveReader, archiveCloser, archiveSize, err := processor.loadArchive(ctx, archiveID)
	if err != nil {
		return fmt.Errorf("load archive: %w", err)
	}
	defer func() {
		if closeErr := archiveCloser.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close archive: %w", closeErr))
		}
	}()

	archive, err := OpenArchive(ctx, archiveReader, archiveSize, processor.config.Limits)
	if err != nil {
		return processor.handleArchiveError(ctx, submissionID, err)
	}

	temporaryDirectory, err := os.MkdirTemp("", "histproject-site-")
	if err != nil {
		return fmt.Errorf("create temporary site directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(temporaryDirectory); removeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary site directory: %w", removeErr))
		}
	}()

	files := archive.Files()
	if err := extractArchive(ctx, temporaryDirectory, files); err != nil {
		return processor.handleArchiveError(ctx, submissionID, err)
	}

	uploadedKeys := make([]string, 0, len(files))
	for _, file := range files {
		objectKey := path.Join(submission.Slug, file.Path)
		if err := processor.uploadFile(ctx, temporaryDirectory, file, objectKey); err != nil {
			return errors.Join(err, processor.deleteUploaded(ctx, append(uploadedKeys, objectKey)))
		}
		uploadedKeys = append(uploadedKeys, objectKey)
	}

	renderURL := siteURL(processor.config.RenderBaseURL, submission.Slug, "index.html")
	screenshot, err := processor.screenshotter.Capture(ctx, renderURL)
	if err != nil {
		return errors.Join(fmt.Errorf("capture site screenshot: %w", err), processor.deleteUploaded(ctx, uploadedKeys))
	}
	if len(screenshot) == 0 {
		return errors.Join(ErrEmptyScreenshot, processor.deleteUploaded(ctx, uploadedKeys))
	}

	screenshotKey := path.Join(submission.Slug, "screenshot.png")
	if err := processor.storage.PutSiteFile(ctx, screenshotKey, bytes.NewReader(screenshot), int64(len(screenshot)), "image/png"); err != nil {
		return errors.Join(fmt.Errorf("upload site screenshot: %w", err), processor.deleteUploaded(ctx, append(uploadedKeys, screenshotKey)))
	}

	previewURL := siteURL(processor.config.PreviewBaseURL, submission.Slug, "index.html")
	screenshotURL := siteURL(processor.config.PublicSiteBaseURL, submission.Slug, "screenshot.png")
	if _, err := processor.submissions.MarkReady(ctx, submissionID, previewURL, screenshotURL); err != nil {
		return fmt.Errorf("mark submission ready: %w", err)
	}
	return nil
}

func (processor *Processor) uploadFile(ctx context.Context, directory string, file File, objectKey string) error {
	source, err := os.Open(filepath.Join(directory, filepath.FromSlash(file.Path)))
	if err != nil {
		return fmt.Errorf("open extracted site file: %w", err)
	}
	contentType := mime.TypeByExtension(path.Ext(file.Path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	uploadErr := processor.storage.PutSiteFile(ctx, objectKey, source, file.Size, contentType)
	closeErr := source.Close()
	if uploadErr != nil {
		uploadErr = fmt.Errorf("upload site file %s: %w", file.Path, uploadErr)
	}
	return errors.Join(uploadErr, closeErr)
}

// Fail rejects a submission whose archive task has run out of attempts.
func (processor *Processor) Fail(ctx context.Context, submissionID int64) error {
	rejectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	_, err := processor.submissions.RejectProcessing(rejectCtx, submissionID, "Не удалось обработать архив")
	if err != nil && !errors.Is(err, submissions.ErrInvalidTransition) {
		return fmt.Errorf("reject failed submission: %w", err)
	}
	return nil
}

func (processor *Processor) handleArchiveError(ctx context.Context, submissionID int64, err error) error {
	if !isArchiveValidationError(err) {
		return err
	}
	reason := "Архив не прошел проверку безопасности"
	if errors.Is(err, ErrIndexMissing) {
		reason = "В корне архива отсутствует index.html"
	}
	if _, rejectErr := processor.submissions.RejectProcessing(ctx, submissionID, reason); rejectErr != nil {
		return errors.Join(err, fmt.Errorf("reject invalid archive: %w", rejectErr))
	}
	return nil
}

func (processor *Processor) deleteUploaded(ctx context.Context, objectKeys []string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var cleanupErr error
	for index := len(objectKeys) - 1; index >= 0; index-- {
		if err := processor.storage.DeleteSiteFile(cleanupCtx, objectKeys[index]); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete uploaded site file %s: %w", objectKeys[index], err))
		}
	}
	return cleanupErr
}

func extractArchive(ctx context.Context, directory string, files []File) error {
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		destination := filepath.Join(directory, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return fmt.Errorf("create site directory: %w", err)
		}
		if err := extractFile(ctx, destination, file); err != nil {
			return fmt.Errorf("extract %s: %w", file.Path, err)
		}
	}
	return nil
}

func extractFile(ctx context.Context, destination string, file File) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	target, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Join(err, source.Close())
	}

	readLimit := file.Size
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	written, copyErr := io.Copy(target, io.LimitReader(contextReader{ctx: ctx, reader: source}, readLimit))
	closeErr := errors.Join(target.Close(), source.Close())
	if errors.Is(copyErr, zip.ErrChecksum) || errors.Is(copyErr, zip.ErrFormat) {
		copyErr = fmt.Errorf("%w: %v", ErrInvalidArchive, copyErr)
	}
	if copyErr != nil || closeErr != nil {
		return errors.Join(copyErr, closeErr)
	}
	if written != file.Size {
		return ErrInvalidArchive
	}
	return nil
}

func isArchiveValidationError(err error) bool {
	return errors.Is(err, ErrArchiveTooLarge) ||
		errors.Is(err, ErrDuplicatePath) ||
		errors.Is(err, ErrExtractedTooLarge) ||
		errors.Is(err, ErrFileTooLarge) ||
		errors.Is(err, ErrIndexMissing) ||
		errors.Is(err, ErrInvalidArchive) ||
		errors.Is(err, ErrSymlink) ||
		errors.Is(err, ErrTooManyFiles) ||
		errors.Is(err, ErrUnsafePath) ||
		errors.Is(err, ErrUnsupportedFile)
}

func siteURL(baseURL, slug, filePath string) string {
	segments := append([]string{slug}, strings.Split(filePath, "/")...)
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.Join(segments, "/")
}

func safeSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for index := 0; index < len(slug); index++ {
		character := slug[index]
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

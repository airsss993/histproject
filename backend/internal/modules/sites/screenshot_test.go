package sites

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChromeScreenshotterUsesBoundedContext(t *testing.T) {
	var capturedURL string
	screenshotter, err := newChromeScreenshotter("http://chrome:9222", 50*time.Millisecond, func(ctx context.Context, siteURL string) ([]byte, error) {
		capturedURL = siteURL
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 50*time.Millisecond {
			t.Fatalf("capture context deadline = %v, ok = %v", deadline, ok)
		}
		return []byte("png"), nil
	})
	if err != nil {
		t.Fatalf("newChromeScreenshotter() error = %v", err)
	}

	image, err := screenshotter.Capture(t.Context(), "http://minio:9000/sites/event/index.html")
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if string(image) != "png" || capturedURL != "http://minio:9000/sites/event/index.html" {
		t.Fatalf("Capture() = image:%q URL:%q", image, capturedURL)
	}
}

func TestChromeScreenshotterHonorsParentCancellation(t *testing.T) {
	screenshotter, err := newChromeScreenshotter("http://chrome:9222", time.Second, func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("newChromeScreenshotter() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = screenshotter.Capture(ctx, "http://minio:9000/sites/event/index.html")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Capture() error = %v, want context.Canceled", err)
	}
}

func TestChromeScreenshotterTimesOut(t *testing.T) {
	screenshotter, err := newChromeScreenshotter("http://chrome:9222", time.Millisecond, func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("newChromeScreenshotter() error = %v", err)
	}

	_, err = screenshotter.Capture(t.Context(), "http://minio:9000/sites/event/index.html")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Capture() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestNewChromeScreenshotterValidatesConfig(t *testing.T) {
	tests := []struct {
		name       string
		browserURL string
		timeout    time.Duration
	}{
		{name: "browser URL", browserURL: "://bad", timeout: time.Second},
		{name: "timeout", browserURL: "http://chrome:9222", timeout: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewChromeScreenshotter(test.browserURL, test.timeout); err == nil {
				t.Fatal("NewChromeScreenshotter() error = nil")
			}
		})
	}
}

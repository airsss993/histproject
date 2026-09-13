package sites

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/chromedp/chromedp"
)

type captureFunc func(context.Context, string) ([]byte, error)

type ChromeScreenshotter struct {
	timeout time.Duration
	capture captureFunc
}

func NewChromeScreenshotter(browserURL string, timeout time.Duration) (*ChromeScreenshotter, error) {
	return newChromeScreenshotter(browserURL, timeout, remoteCapture(browserURL))
}

func newChromeScreenshotter(browserURL string, timeout time.Duration, capture captureFunc) (*ChromeScreenshotter, error) {
	parsedURL, err := url.Parse(browserURL)
	if err != nil || parsedURL.Host == "" || !validBrowserScheme(parsedURL.Scheme) {
		return nil, fmt.Errorf("invalid Chrome URL %q", browserURL)
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("invalid screenshot timeout %s", timeout)
	}
	return &ChromeScreenshotter{timeout: timeout, capture: capture}, nil
}

func (screenshotter *ChromeScreenshotter) Capture(ctx context.Context, siteURL string) ([]byte, error) {
	captureCtx, cancel := context.WithTimeout(ctx, screenshotter.timeout)
	defer cancel()
	image, err := screenshotter.capture(captureCtx, siteURL)
	if err != nil {
		return nil, fmt.Errorf("capture screenshot: %w", err)
	}
	return image, nil
}

func remoteCapture(browserURL string) captureFunc {
	return func(ctx context.Context, siteURL string) ([]byte, error) {
		allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(ctx, browserURL)
		defer cancelAllocator()
		tabCtx, cancelTab := chromedp.NewContext(allocatorCtx)
		defer cancelTab()

		var image []byte
		if err := chromedp.Run(tabCtx,
			chromedp.EmulateViewport(1920, 1080),
			chromedp.Navigate(siteURL),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.CaptureScreenshot(&image),
		); err != nil {
			return nil, err
		}
		return image, nil
	}
}

func validBrowserScheme(scheme string) bool {
	return scheme == "http" || scheme == "https" || scheme == "ws" || scheme == "wss"
}

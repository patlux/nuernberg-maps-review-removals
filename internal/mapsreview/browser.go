package mapsreview

import (
	"context"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36"

func NewRemoteBrowserContext(cdpURL string) (context.Context, context.CancelFunc) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), cdpURL)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	_ = chromedp.Run(browserCtx)
	return browserCtx, func() {
		browserCancel()
		allocCancel()
	}
}

func NewBrowserContext(headless bool) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("lang", "de-DE"),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.UserAgent(UserAgent),
		chromedp.WindowSize(1440, 1100),
	)
	if path := chromeExecPath(); path != "" {
		opts = append(opts, chromedp.ExecPath(path))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	_ = chromedp.Run(browserCtx,
		network.Enable(),
		network.SetBlockedURLs([]string{
			"*.png*", "*.jpg*", "*.jpeg*", "*.gif*", "*.webp*", "*.avif*", "*.ico*",
			"*.woff*", "*.woff2*", "*.ttf*", "*.otf*",
			"*.mp4*", "*.webm*", "*.m4s*", "*.mp3*",
		}),
	)
	return browserCtx, func() {
		browserCancel()
		allocCancel()
	}
}

func chromeExecPath() string {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func RunWithTimeout(parent context.Context, timeout time.Duration, actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

package mapsreview

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

const UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36"

func NewBrowserContext(headless bool) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("lang", "de-DE"),
		chromedp.UserAgent(UserAgent),
		chromedp.WindowSize(1440, 1100),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	return browserCtx, func() {
		browserCancel()
		allocCancel()
	}
}

func RunWithTimeout(parent context.Context, timeout time.Duration, actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

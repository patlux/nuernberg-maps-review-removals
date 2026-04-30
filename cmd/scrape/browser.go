package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/chromedp/chromedp"
	"nuernberg-maps-review-removals/internal/mapsreview"
)

type discoveredAnchor struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type mapText struct {
	Title string `json:"title"`
	H1    string `json:"h1"`
	Text  string `json:"text"`
}

func urlPathEscape(value string) string {
	return url.PathEscape(value)
}

func navigate(ctx context.Context, rawURL string, timeout time.Duration) error {
	return mapsreview.RunWithTimeout(ctx, timeout,
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
}

func waitForPlacePanel(ctx context.Context) error {
	var ready bool
	err := mapsreview.RunWithTimeout(ctx, 12*time.Second, chromedp.Poll(`(() => {
  const text = document.body?.innerText || '';
  const consent = /Bevor Sie zu Google weitergehen|Before you go to Google|Alle akzeptieren|Accept all/i.test(text);
  const hasTitle = Boolean(document.querySelector('h1')?.textContent?.trim()) || /Google Maps/i.test(document.title || '');
  return !consent && hasTitle && text.length > 500;
})()`, &ready, chromedp.WithPollingInterval(150*time.Millisecond), chromedp.WithPollingTimeout(10*time.Second)))
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("place panel did not become ready")
	}
	return nil
}

func acceptConsent(ctx context.Context) error {
	var accepted bool
	err := mapsreview.RunWithTimeout(ctx, 5*time.Second, chromedp.Evaluate(`(() => {
  const patterns = [/Alle akzeptieren/i, /Ich stimme zu/i, /Akzeptieren/i, /Accept all/i, /I agree/i];
  const buttons = Array.from(document.querySelectorAll('button, [role="button"], input[type="submit"]'));
  const button = buttons.find(el => patterns.some(pattern => pattern.test(el.innerText || el.textContent || el.value || el.getAttribute('aria-label') || '')));
  if (!button) return false;
  button.click();
  return true;
})()`, &accepted))
	if accepted {
		sleep(1000)
	}
	return err
}

func readPlaceAnchors(ctx context.Context) ([]discoveredAnchor, error) {
	var anchors []discoveredAnchor
	err := mapsreview.RunWithTimeout(ctx, 10*time.Second, chromedp.Evaluate(`(() => {
  const places = Array.from(document.querySelectorAll('a[href*="/maps/place/"]')).map(a => ({
    url: a.href,
    name: (a.getAttribute('aria-label') || a.textContent || '').split('\n')[0].trim()
  })).filter(place => place.url);
  if (location.href.includes('/maps/place/')) {
    const name = document.querySelector('h1')?.textContent?.trim() || document.title.replace(/\s*-\s*Google Maps.*/i, '').trim();
    if (name) places.unshift({ url: location.href, name });
  }
  const seen = new Set();
  return places.filter(place => {
    if (seen.has(place.url)) return false;
    seen.add(place.url);
    return true;
  });
})()`, &anchors))
	return anchors, err
}

func scrollResults(ctx context.Context) error {
	var ignored any
	return mapsreview.RunWithTimeout(ctx, 5*time.Second, chromedp.Evaluate(`(() => {
  const feed = document.querySelector('div[role="feed"]') || document.querySelector('[aria-label*="Ergebnisse"]') || document.scrollingElement;
  if (feed && feed.scrollBy) feed.scrollBy(0, 2400);
  else window.scrollBy(0, 2400);
})()`, &ignored))
}

func readMapText(ctx context.Context) (mapText, error) {
	var out mapText
	err := mapsreview.RunWithTimeout(ctx, 10*time.Second, chromedp.Evaluate(`(() => {
  const attrTexts = Array.from(document.querySelectorAll('[aria-label], [alt], [data-tooltip]'))
    .flatMap(el => [el.getAttribute('aria-label'), el.getAttribute('alt'), el.getAttribute('data-tooltip')])
    .filter(Boolean);
  return {
    title: document.title,
    h1: document.querySelector('h1')?.textContent?.trim() || '',
    text: [document.body.innerText, ...attrTexts].join('\n')
  };
})()`, &out))
	return out, err
}

func clickReviewsTab(ctx context.Context) bool {
	var clicked bool
	_ = mapsreview.RunWithTimeout(ctx, 5*time.Second, chromedp.Evaluate(`(() => {
  const candidates = Array.from(document.querySelectorAll('button, [role="tab"], [role="button"]'));
  const el = candidates.find(el => /Rezensionen|Reviews/i.test(el.innerText || el.textContent || el.getAttribute('aria-label') || ''));
  if (!el) return false;
  el.click();
  return true;
})()`, &clicked))
	if clicked {
		_ = waitForReviewsPanel(ctx)
	}
	return clicked
}

func waitForReviewsPanel(ctx context.Context) error {
	var ready bool
	return mapsreview.RunWithTimeout(ctx, 2500*time.Millisecond, chromedp.Poll(`(() => {
  const text = document.body?.innerText || '';
  return /Sortieren|Weitere Rezensionen|Rezensionen werden nicht überprüft|Reviews are not verified|Ansicht ist beschränkt|limited view/i.test(text);
})()`, &ready, chromedp.WithPollingInterval(100*time.Millisecond), chromedp.WithPollingTimeout(2*time.Second)))
}

func screenshot(ctx context.Context, file string) error {
	var buf []byte
	if err := mapsreview.RunWithTimeout(ctx, 15*time.Second, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return err
	}
	if err := mapsreview.EnsureDirForPath(file); err != nil {
		return err
	}
	return os.WriteFile(file, buf, 0o644)
}

func safeFilename(value string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(value, "_")
}

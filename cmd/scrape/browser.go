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
	Title       string   `json:"title"`
	H1          string   `json:"h1"`
	Category    string   `json:"category"`
	Text        string   `json:"text"`
	Rating      *float64 `json:"rating"`
	ReviewCount *int     `json:"reviewCount"`
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
	return mapsreview.AcceptConsent(ctx)
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
  const category = Array.from(document.querySelectorAll('button[jsaction*="category"]'))
    .map(el => (el.innerText || el.textContent || '').trim())
    .find(Boolean) || '';

  // Extract rating and review count from structured DOM near h1
  let rating = null, reviewCount = null;
  const h1 = document.querySelector('h1');
  if (h1) {
    const placeSection = h1.closest('[jsaction]') || h1.parentElement;
    // Rating: aria-label like "5,0 Sterne" or "4,5 stars"
    const allRatingLabels = Array.from(document.querySelectorAll('[aria-label*="Sterne" i], [aria-label*="stars" i], [aria-label*="★"]'))
      .map(el => el.getAttribute('aria-label') || '');
    // Prefer the one closest to h1 (not in suggestions/reviews section)
    const ownRating = allRatingLabels.find(label => {
      const el = [...document.querySelectorAll('[aria-label="' + label.replace(/"/g, '') + '" i]')][0];
      if (!el) return true;
      const dist = Math.abs((el.getBoundingClientRect().top || 0) - (h1.getBoundingClientRect().top || 0));
      return dist < 300;
    });
    if (ownRating) {
      const match = ownRating.match(/([\d,]+)\s*(?:Sterne|stars)/i);
      if (match) rating = match[1].replace(',', '.');
    }
    // Review count: aria-label like "1 Rezension" or "173 reviews"
    const reviewLabels = Array.from(document.querySelectorAll('[aria-label*="Rezension" i], [aria-label*="review" i]'))
      .map(el => ({ el, label: el.getAttribute('aria-label') || '', text: el.textContent || '' }));
    const ownReview = reviewLabels.find(r => {
      if (!r.el) return false;
      const dist = Math.abs((r.el.getBoundingClientRect().top || 0) - (h1.getBoundingClientRect().top || 0));
      return dist < 300;
    });
    if (ownReview) {
      const match = ownReview.label.match(/([\d.]+)\s*Rezension/i) || ownReview.text.match(/([\d]+)/);
      if (match) reviewCount = match[1].replace(/\./g, '');
    }
  }

  return {
    title: document.title,
    h1: document.querySelector('h1')?.textContent?.trim() || '',
    category,
    text: [document.body.innerText, ...attrTexts].join('\n'),
    rating: rating ? parseFloat(rating) : null,
    reviewCount: reviewCount ? parseInt(reviewCount, 10) : null
  };
})()`, &out))
	return out, err
}

func waitForDirectReviewsPanel(ctx context.Context) error {
	var ready bool
	return mapsreview.RunWithTimeout(ctx, 15*time.Second, chromedp.Poll(`(() => {
  const text = document.body?.innerText || '';
  const hasReviewsPanel = /Sortieren|In Rezensionen suchen|Berichte|Bewertungen aufgrund|Diffamierung|No reviews|Noch keine Rezensionen|Bevor Sie zu Google weitergehen|Before you go to Google/i.test(text);
  if (hasReviewsPanel) return true;
  const hasLoadedPlace = text.length > 450 && (Boolean(document.querySelector('h1')?.textContent?.trim()) || /Google Maps/i.test(document.title || ''));
  const hasNoReviewsTab = !/(^|\n)Rezensionen(\n|$)|Berichte|Bewertungen aufgrund|Diffamierung|Reviews/i.test(text);
  const hasPlaceActions = /Rezension schreiben|Fotos und Videos|Routen.{0,10}planer|Write a review|Photos and videos|Directions/i.test(text);
  const noReviewsCandidate = hasLoadedPlace && hasNoReviewsTab && hasPlaceActions;
  if (!noReviewsCandidate) {
    window.__mapsReviewNoReviewsSince = 0;
    return false;
  }
  window.__mapsReviewNoReviewsSince ||= Date.now();
  return Date.now() - window.__mapsReviewNoReviewsSince > 4500;
})()`, &ready, chromedp.WithPollingInterval(150*time.Millisecond), chromedp.WithPollingTimeout(12*time.Second)))
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

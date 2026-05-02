package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

var errPartialMapsShell = errors.New("partial Google Maps shell")

func main() {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := run(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args args) error {
	if err := mapsreview.EnsureDirForPath(mapsreview.OutputDir); err != nil {
		return err
	}
	if err := mapsreview.EnsureDirForPath("debug"); err != nil {
		return err
	}

	browserCtx, cancel := newScrapeBrowserContext(args)
	defer cancel()

	discoveries := []mapsreview.Discovery{}
	var err error
	if args.ScrapeOnly {
		discoveries, err = mapsreview.ReadJSON(mapsreview.DiscoveryJSON, []mapsreview.Discovery{})
	} else {
		discoveries, err = discoverPlaces(browserCtx, args)
	}
	if err != nil {
		return err
	}

	if args.DiscoveryOnly {
		if err := writeMetadata(args, discoveries, nil); err != nil {
			return err
		}
		fmt.Printf("Discovery complete: %d places\n", len(discoveries))
		return nil
	}

	rows, err := scrapePlaces(browserCtx, discoveries, args)
	if err != nil {
		return err
	}
	if err := writeMetadata(args, discoveries, rows); err != nil {
		return err
	}
	fmt.Printf("\nDone. Results: %s and %s\n", args.Out, args.CSV)
	return nil
}

func newScrapeBrowserContext(args args) (context.Context, context.CancelFunc) {
	if args.CDPURL != "" {
		return mapsreview.NewRemoteBrowserContext(args.CDPURL)
	}
	return mapsreview.NewBrowserContext(args.Headless)
}

type metadata struct {
	ReadAt            string   `json:"readAt"`
	Postcodes         []string `json:"postcodes"`
	Queries           []string `json:"queries"`
	MaxResults        int      `json:"maxResults"`
	Headless          bool     `json:"headless"`
	CDPURL            string   `json:"cdpUrl,omitempty"`
	DiscoveryOnly     bool     `json:"discoveryOnly"`
	ScrapeOnly        bool     `json:"scrapeOnly"`
	RescrapeAll       bool     `json:"rescrapeAll"`
	BannerAuditOnly   bool     `json:"bannerAuditOnly"`
	AllowBannerClears bool     `json:"allowBannerClears"`
	NoticeAttempts    int      `json:"noticeAttempts"`
	ScrapeStart       int      `json:"scrapeStart"`
	ScrapeLimit       int      `json:"scrapeLimit"`
	SaveEvery         int      `json:"saveEvery"`
	DelayMin          int      `json:"delayMin"`
	DelayMax          int      `json:"delayMax"`
	Output            string   `json:"output"`
	CSV               string   `json:"csv"`
	UserAgent         string   `json:"userAgent"`
	Discovered        int      `json:"discovered"`
	Rows              int      `json:"rows"`
	Success           int      `json:"success"`
	Errors            int      `json:"errors"`
}

func writeMetadata(args args, discoveries []mapsreview.Discovery, rows []mapsreview.Place) error {
	m := metadata{
		ReadAt:            mapsreview.NowISO(),
		Postcodes:         args.Postcodes,
		Queries:           args.Queries,
		MaxResults:        args.MaxResults,
		Headless:          args.Headless,
		CDPURL:            args.CDPURL,
		DiscoveryOnly:     args.DiscoveryOnly,
		ScrapeOnly:        args.ScrapeOnly,
		RescrapeAll:       args.RescrapeAll,
		BannerAuditOnly:   args.BannerAuditOnly,
		AllowBannerClears: args.AllowBannerClears,
		NoticeAttempts:    args.NoticeAttempts,
		ScrapeStart:       args.ScrapeStart,
		ScrapeLimit:       args.ScrapeLimit,
		SaveEvery:         args.SaveEvery,
		DelayMin:          args.DelayMin,
		DelayMax:          args.DelayMax,
		Output:            args.Out,
		CSV:               args.CSV,
		UserAgent:         mapsreview.UserAgent,
		Discovered:        len(discoveries),
		Rows:              len(rows),
	}
	for _, row := range rows {
		if row.Status == "success" {
			m.Success++
		} else if row.Status != "" {
			m.Errors++
		}
	}
	return mapsreview.WriteJSON(mapsreview.MetadataJSON, m)
}

func discoverPlaces(ctx context.Context, args args) ([]mapsreview.Discovery, error) {
	existing, err := mapsreview.ReadJSON(mapsreview.DiscoveryJSON, []mapsreview.Discovery{})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	discoveries := make([]mapsreview.Discovery, 0, len(existing))
	for _, place := range existing {
		if place.ID == "" || seen[place.ID] {
			continue
		}
		seen[place.ID] = true
		discoveries = append(discoveries, place)
	}

	stop := false
	for _, postcode := range args.Postcodes {
		for _, query := range args.Queries {
			if args.MaxResults > 0 && len(discoveries) >= args.MaxResults {
				stop = true
				break
			}
			search := fmt.Sprintf("%s %s Nürnberg", query, postcode)
			url := "https://www.google.com/maps/search/" + urlPathEscape(search) + "?hl=de"
			fmt.Printf("\nDiscover: %s\n", search)
			if err := navigate(ctx, url, 60*time.Second); err != nil {
				return nil, err
			}
			_ = acceptConsent(ctx)
			sleep(3000)

			lastCount := -1
			stagnant := 0
			for pass := 0; pass < 45; pass++ {
				anchors, err := readPlaceAnchors(ctx)
				if err != nil {
					return nil, err
				}
				for _, anchor := range anchors {
					id := mapsreview.PlaceIDFromURL(anchor.URL)
					if !seen[id] {
						seen[id] = true
						discoveries = append(discoveries, mapsreview.Discovery{
							ID:                 id,
							Name:               anchor.Name,
							URL:                mapsreview.NormalizeURL(anchor.URL),
							DiscoveredPostcode: postcode,
							DiscoveredQuery:    query,
						})
					}
				}

				if len(discoveries) == lastCount {
					stagnant++
				} else {
					stagnant = 0
				}
				lastCount = len(discoveries)
				fmt.Printf("\r  places: %d   ", len(discoveries))

				if args.MaxResults > 0 && len(discoveries) >= args.MaxResults {
					break
				}
				if stagnant >= 5 {
					break
				}
				_ = scrollResults(ctx)
				sleep(1400)
			}
			if err := mapsreview.WriteJSON(mapsreview.DiscoveryJSON, discoveries); err != nil {
				return nil, err
			}
			fmt.Printf("\n  saved %d discoveries\n", len(discoveries))
		}
		if stop {
			break
		}
	}
	if args.MaxResults > 0 && len(discoveries) > args.MaxResults {
		discoveries = discoveries[:args.MaxResults]
	}
	return discoveries, nil
}

func extractPlace(ctx context.Context, discovery mapsreview.Discovery) (mapsreview.Place, error) {
	if discovery.URL == "" {
		return mapsreview.Place{}, errors.New("missing URL")
	}
	overview, overviewErr := extractOverview(ctx, discovery)
	reviews, err := extractReviewsDirectWithRetry(ctx, discovery)
	if err != nil {
		if overviewErr != nil {
			return mapsreview.Place{}, fmt.Errorf("%v; overview error: %v", err, overviewErr)
		}
		return mapsreview.Place{}, err
	}

	rawTitle := reviews.Title
	if rawTitle == "" {
		rawTitle = overview.Title
	}
	rawH1 := reviews.H1
	if rawH1 == "" {
		rawH1 = overview.H1
	}
	rawText := combinedMapText(overview, reviews)
	if isConsentPage(rawText) {
		return mapsreview.Place{}, errors.New("Google consent page still visible")
	}
	if isRestrictedMapsView(reviews.Text) {
		return mapsreview.Place{}, errors.New("restricted Google Maps view")
	}
	name := rawH1
	if name == "" {
		name = discovery.Name
	}
	if name == "" {
		name = regexp.MustCompile(`(?i) - Google Maps.*`).ReplaceAllString(rawTitle, "")
		name = strings.TrimSpace(name)
	}

	statsText := reviews.Text
	// Prefer structured DOM extraction over text parsing
	stats := mapsreview.PlaceStats{Rating: reviews.Rating, ReviewCount: reviews.ReviewCount}
	if stats.Rating == nil || stats.ReviewCount == nil {
		parsed := mapsreview.ParsePlaceStats(statsText)
		if stats.Rating == nil {
			stats.Rating = parsed.Rating
		}
		if stats.ReviewCount == nil {
			stats.ReviewCount = parsed.ReviewCount
		}
	}
	if stats.Rating == nil || stats.ReviewCount == nil {
		fallbackStats := mapsreview.ParsePlaceStats(rawText)
		if stats.Rating == nil {
			stats.Rating = fallbackStats.Rating
		}
		if stats.ReviewCount == nil {
			stats.ReviewCount = fallbackStats.ReviewCount
		}
	}
	placeState := classifyPlaceState(rawText, statsText)
	if placeState == mapsreview.PlaceStateNoPublicReviews {
		stats.Rating = nil
		stats.ReviewCount = mapsreview.IntPtr(0)
	}
	address := mapsreview.ExtractAddress(overview.Text)
	postcode := mapsreview.StringPtr(discovery.DiscoveredPostcode)
	if address != nil {
		if pc := mapsreview.ExtractPostcode(*address); pc != nil {
			postcode = pc
		}
	}
	category := extractCategory(name, overview.Category, overview.Text)
	notice := mapsreview.ParseNotice(statsText)
	coords := mapsreview.ExtractCoordinates(discovery.URL)

	row := mapsreview.Place{
		ID:          discovery.ID,
		Name:        name,
		Postcode:    postcode,
		Address:     address,
		Rating:      stats.Rating,
		ReviewCount: stats.ReviewCount,
		Category:    category,
		URL:         mapsreview.NormalizeURL(discovery.URL),
		ReadAt:      mapsreview.NowISO(),
		PlaceState:  placeState,
		Status:      "success",
	}
	if coords != nil {
		row.Lat = mapsreview.FloatPtr(coords.Lat)
		row.Lng = mapsreview.FloatPtr(coords.Lng)
	}
	mapsreview.EnrichPlaceLocation(&row)
	applyNotice(&row, notice)
	mapsreview.ApplyPlaceOverrides(&row)
	mapsreview.ComputeMetrics(&row)
	return row, nil
}

func extractOverview(ctx context.Context, discovery mapsreview.Discovery) (mapText, error) {
	if err := navigate(ctx, mapsreview.NormalizeURL(discovery.URL), 60*time.Second); err != nil {
		return mapText{}, err
	}
	_ = acceptConsent(ctx)
	if err := waitForPlacePanel(ctx); err != nil {
		return mapText{}, err
	}
	overview, err := readMapText(ctx)
	if err != nil {
		return mapText{}, err
	}
	return overview, nil
}

func combinedMapText(overview, reviews mapText) string {
	return overview.Text + "\n" + reviews.Text
}

func applyNotice(row *mapsreview.Place, notice *mapsreview.Notice) {
	if notice == nil {
		return
	}
	row.HasDefamationNotice = true
	row.RemovedMin = mapsreview.IntPtr(notice.Min)
	row.RemovedMax = notice.Max
	row.RemovedEstimate = mapsreview.FloatPtr(notice.Estimate)
	row.RemovedText = mapsreview.StringPtr(notice.Text)
}

func applyStatsIfPresent(row *mapsreview.Place, stats mapsreview.PlaceStats) {
	if stats.Rating != nil {
		row.Rating = stats.Rating
	}
	if stats.ReviewCount != nil {
		row.ReviewCount = stats.ReviewCount
	}
}

func extractReviewsDirectWithRetry(ctx context.Context, discovery mapsreview.Discovery) (mapText, error) {
	reviews, err := extractReviewsDirect(ctx, discovery)
	if err == nil || errors.Is(err, context.Canceled) {
		return reviews, err
	}
	firstErr := err
	sleep(500)
	reviews, err = extractReviewsDirect(ctx, discovery)
	if err != nil {
		return mapText{}, fmt.Errorf("direct reviews failed after retry: %w; first error: %v", err, firstErr)
	}
	return reviews, nil
}

func extractNoticeWithAttempts(ctx context.Context, discovery mapsreview.Discovery, attempts int) (*mapsreview.Notice, mapsreview.PlaceStats, []string, error) {
	attempts = max(1, attempts)
	texts := []string{}
	var lastStats mapsreview.PlaceStats
	var lastErr error
	gotText := false
	for attempt := 1; attempt <= attempts; attempt++ {
		reviews, err := extractReviewsDirectWithRetry(ctx, discovery)
		if err != nil {
			lastErr = err
			texts = append(texts, fmt.Sprintf("attempt %d error: %v", attempt, err))
			if errors.Is(err, context.Canceled) {
				return nil, lastStats, texts, err
			}
		} else {
			gotText = true
			texts = append(texts, reviews.Text)
			lastStats = mapsreview.ParsePlaceStats(reviews.Text)
			if notice := mapsreview.ParseNotice(reviews.Text); notice != nil {
				return notice, lastStats, texts, nil
			}
		}
		if attempt < attempts {
			sleep(750)
		}
	}
	if !gotText && lastErr != nil {
		return nil, lastStats, texts, lastErr
	}
	return nil, lastStats, texts, nil
}

func writeNoticeDebug(discovery mapsreview.Discovery, texts []string, err error) error {
	path := filepath.Join("debug", "banner-clear-"+safeFilename(discovery.ID)+".txt")
	if err := mapsreview.EnsureDirForPath(path); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("place: " + displayPlaceName(discovery) + "\n")
	b.WriteString("id: " + discovery.ID + "\n")
	b.WriteString("url: " + mapsreview.NormalizeURL(discovery.URL) + "\n")
	b.WriteString("readAt: " + mapsreview.NowISO() + "\n")
	if err != nil {
		b.WriteString("error: " + err.Error() + "\n")
	}
	for i, text := range texts {
		b.WriteString(fmt.Sprintf("\n--- attempt %d (%d bytes) ---\n", i+1, len(text)))
		b.WriteString(text)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func extractReviewsDirect(ctx context.Context, discovery mapsreview.Discovery) (mapText, error) {
	reviewsURL := mapsreview.ReviewsURLFromURL(discovery.URL)
	if reviewsURL == mapsreview.NormalizeURL(discovery.URL) {
		return mapText{}, errors.New("restricted Google Maps view")
	}
	reviews, err := extractReviewsDirectOnce(ctx, reviewsURL, discovery)
	if err == nil || !errors.Is(err, errPartialMapsShell) {
		return reviews, err
	}
	sleep(1000)
	return extractReviewsDirectOnce(ctx, reviewsURL, discovery)
}

func extractReviewsDirectOnce(ctx context.Context, reviewsURL string, discovery mapsreview.Discovery) (mapText, error) {
	if err := navigate(ctx, reviewsURL, 60*time.Second); err != nil {
		return mapText{}, err
	}
	_ = acceptConsent(ctx)
	if err := waitForDirectReviewsPanel(ctx); err != nil {
		reviews, readErr := readMapText(ctx)
		if readErr == nil && isPartialMapsShell(reviews.Text, discovery.Name) {
			return reviews, fmt.Errorf("%w: %v", errPartialMapsShell, err)
		}
		return reviews, err
	}
	reviews, err := readMapText(ctx)
	if err != nil {
		return reviews, err
	}
	if isPartialMapsShell(reviews.Text, discovery.Name) {
		return reviews, errPartialMapsShell
	}
	return reviews, nil
}

func isConsentPage(text string) bool {
	compact := strings.ToLower(strings.Join(strings.Fields(text), " "))
	return strings.Contains(compact, "bevor sie zu google weitergehen") || strings.Contains(compact, "before you go to google")
}

func classifyPlaceState(rawText, directReviewsText string) string {
	compact := strings.ToLower(strings.Join(strings.Fields(rawText), " "))
	switch {
	case strings.Contains(compact, "dauerhaft geschlossen") || strings.Contains(compact, "permanently closed"):
		return mapsreview.PlaceStatePermanentlyClosed
	case strings.Contains(compact, "vorübergehend geschlossen") || strings.Contains(compact, "temporarily closed"):
		return mapsreview.PlaceStateTemporarilyClosed
	case directReviewsTextHasNoPublicReviews(directReviewsText):
		return mapsreview.PlaceStateNoPublicReviews
	default:
		return mapsreview.PlaceStateActive
	}
}

func directReviewsTextHasNoPublicReviews(text string) bool {
	text = mapsreview.TrimMapsAncillarySections(text)
	compact := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if strings.Contains(compact, "noch keine rezensionen") || strings.Contains(compact, "no reviews") {
		return true
	}
	if regexp.MustCompile(`(?im)(^|\n)\s*Rezensionen\s*(\n|$)`).MatchString(text) {
		return false
	}
	reviewPanelMarkers := []string{"sortieren", "in rezensionen suchen", "berichte", "bewertungen aufgrund", "diffamierung"}
	for _, marker := range reviewPanelMarkers {
		if strings.Contains(compact, marker) {
			return false
		}
	}
	canWriteReview := strings.Contains(compact, "rezension schreiben") || strings.Contains(compact, "write a review")
	loadedPlace := strings.Contains(compact, "übersicht") || strings.Contains(compact, "overview") || strings.Contains(compact, "routenplaner") || strings.Contains(compact, "directions")
	return canWriteReview && loadedPlace
}

func isPartialMapsShell(text, expectedName string) bool {
	compact := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if isConsentPage(text) || directReviewsTextHasNoPublicReviews(text) || compact == "" || len(compact) > 800 {
		return false
	}
	if expectedName != "" && strings.Contains(compact, strings.ToLower(strings.Join(strings.Fields(expectedName), " "))) {
		return false
	}
	hasMapsShell := strings.Contains(compact, "gespeichert") && strings.Contains(compact, "zuletzt verwendet")
	hasNoPlaceContent := !strings.Contains(compact, "rezension schreiben") && !strings.Contains(compact, "sortieren") && !strings.Contains(compact, "berichte") && !strings.Contains(compact, "diffamierung")
	return hasMapsShell && hasNoPlaceContent
}

func isRestrictedMapsView(text string) bool {
	compact := strings.ToLower(strings.Join(strings.Fields(text), " "))
	hasRestrictedMarker := strings.Contains(compact, "ansicht ist beschränkt") || strings.Contains(compact, "limited view")
	if !hasRestrictedMarker {
		return false
	}
	usablePlaceMarkers := []string{
		"rezension schreiben",
		"sortieren",
		"berichte",
		"bewertungen aufgrund",
		"noch keine rezensionen",
		"routenplaner",
		"fotos und videos",
		"write a review",
		"directions",
		"photos and videos",
	}
	for _, marker := range usablePlaceMarkers {
		if strings.Contains(compact, marker) {
			return false
		}
	}
	return true
}

func extractCategory(name, domCategory, text string) *string {
	if category := cleanCategoryCandidate(domCategory, name); category != "" {
		return mapsreview.StringPtr(category)
	}
	window := text
	if name != "" {
		if idx := strings.Index(strings.ToLower(text), strings.ToLower(name)); idx >= 0 {
			window = text[idx+len(name):]
		}
	}
	if len(window) > 800 {
		window = window[:800]
	}
	for _, stop := range []string{"\nÜbersicht\n", "\nOverview\n", "\nRezensionen\n", "\nReviews\n", "\nInfo\n", "\nRouten"} {
		if idx := strings.Index(strings.ToLower(window), strings.ToLower(stop)); idx >= 0 {
			window = window[:idx]
		}
	}
	for _, line := range strings.Split(window, "\n") {
		if category := cleanCategoryCandidate(line, name); category != "" {
			return mapsreview.StringPtr(category)
		}
	}
	return nil
}

func cleanCategoryCandidate(value, name string) string {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return ""
	}
	if idx := strings.Index(candidate, "·"); idx >= 0 {
		candidate = candidate[:idx]
	}
	candidate = strings.TrimFunc(candidate, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	lower := strings.ToLower(candidate)
	if name != "" && lower == strings.ToLower(strings.TrimSpace(name)) {
		return ""
	}
	blocked := map[string]bool{
		"restaurants in der nähe":  true,
		"restaurants in der naehe": true,
		"hotels":                   true,
		"mögliche aktivitäten":     true,
		"moegliche aktivitaeten":   true,
		"bars":                     true,
		"kaffee":                   true,
		"zum mitnehmen":            true,
		"lebensmittel":             true,
		"gespeichert":              true,
		"zuletzt verwendet":        true,
		"app herunterladen":        true,
		"fotos ansehen":            true,
		"übersicht":                true,
		"speisekarte":              true,
		"rezensionen":              true,
		"info":                     true,
		"routenplaner":             true,
		"speichern":                true,
		"in der nähe":              true,
		"teilen":                   true,
	}
	if blocked[lower] {
		return ""
	}
	if regexp.MustCompile(`(?i)^[1-5](?:[,.][0-9])?$|^\(?[0-9][0-9.]*\)?$|€|geöffnet|geschlossen|adresse|telefon|website|\.de\b|\b9\d{4}\b`).MatchString(candidate) {
		return ""
	}
	if !regexp.MustCompile(`\p{L}`).MatchString(candidate) {
		return ""
	}
	return candidate
}

func scrapePlaces(ctx context.Context, discoveries []mapsreview.Discovery, args args) ([]mapsreview.Place, error) {
	previous, err := mapsreview.ReadJSON(args.Out, []mapsreview.Place{})
	if err != nil {
		return nil, err
	}
	rows := map[string]mapsreview.Place{}
	for _, row := range previous {
		rows[row.ID] = row
	}
	if args.BannerAuditOnly {
		return auditBannerPlaces(ctx, discoveries, args, rows)
	}

	todo := make([]mapsreview.Discovery, 0, len(discoveries))
	for _, place := range discoveries {
		row, ok := rows[place.ID]
		if args.RescrapeAll || !ok || row.Status != "success" {
			todo = append(todo, place)
		}
	}
	if args.ScrapeStart > 1 {
		if args.ScrapeStart > len(todo) {
			todo = nil
		} else {
			todo = todo[args.ScrapeStart-1:]
		}
	}
	if args.ScrapeLimit > 0 && args.ScrapeLimit < len(todo) {
		todo = todo[:args.ScrapeLimit]
	}
	fmt.Printf("\nScrape: %d remaining / %d discovered", len(todo), len(discoveries))
	if args.ScrapeStart > 1 {
		fmt.Printf(" (starting at todo position %d)", args.ScrapeStart)
	}
	if args.ScrapeLimit > 0 {
		fmt.Printf(" (limit %d)", args.ScrapeLimit)
	}
	fmt.Println()

	changedSinceSave := 0
	saveEvery := max(1, args.SaveEvery)
	saveIfNeeded := func(force bool) error {
		if changedSinceSave == 0 || (!force && changedSinceSave < saveEvery) {
			return nil
		}
		if err := saveRows(args, rows); err != nil {
			return err
		}
		changedSinceSave = 0
		return nil
	}

	for i, place := range todo {
		fmt.Printf("[%d/%d] %s\n", i+1, len(todo), displayPlaceName(place))
		previousRow, hadPreviousRow := rows[place.ID]
		row, err := extractPlace(ctx, place)
		if err != nil {
			errorText := err.Error()
			if hadPreviousRow && previousRow.Status == "success" {
				fmt.Printf("  ERROR: %s; keeping existing success row\n", errorText)
				if errors.Is(err, context.Canceled) {
					if saveErr := saveIfNeeded(true); saveErr != nil {
						return nil, saveErr
					}
					return mapValues(rows), err
				}
				sleepBetweenPlaces(i, len(todo), args)
				continue
			}
			if errors.Is(err, context.Canceled) {
				if saveErr := saveIfNeeded(true); saveErr != nil {
					return nil, saveErr
				}
				return mapValues(rows), err
			}
			placeState := ""
			if errors.Is(err, errPartialMapsShell) {
				placeState = mapsreview.PlaceStatePartialLoad
			}
			row = mapsreview.Place{
				ID:         place.ID,
				Name:       place.Name,
				Postcode:   mapsreview.StringPtr(place.DiscoveredPostcode),
				URL:        mapsreview.NormalizeURL(place.URL),
				ReadAt:     mapsreview.NowISO(),
				PlaceState: placeState,
				Status:     "error",
				Error:      mapsreview.StringPtr(errorText),
			}
			fmt.Printf("  ERROR: %s\n", errorText)
			_ = screenshot(ctx, filepath.Join("debug", safeFilename(place.ID)+".png"))
		} else {
			if hadPreviousRow {
				row = preservePreviousMetadata(previousRow, row)
				if previousRow.HasDefamationNotice && !row.HasDefamationNotice && !args.AllowBannerClears {
					notice, stats, texts, verifyErr := extractNoticeWithAttempts(ctx, place, args.NoticeAttempts)
					if notice != nil {
						applyStatsIfPresent(&row, stats)
						applyNotice(&row, notice)
						mapsreview.ComputeMetrics(&row)
						fmt.Printf("  VERIFIED existing banner after extra check: %s\n", notice.Text)
					} else {
						_ = writeNoticeDebug(place, texts, verifyErr)
						_ = screenshot(ctx, filepath.Join("debug", "banner-clear-"+safeFilename(place.ID)+".png"))
					}
				}
			}
			if keep, reason := shouldKeepPreviousRow(previousRow, row, hadPreviousRow, args); keep {
				fmt.Printf("  SKIP: %s; keeping existing success row\n", reason)
				sleepBetweenPlaces(i, len(todo), args)
				continue
			}
			removed := "none"
			if row.RemovedText != nil {
				removed = *row.RemovedText
			}
			fmt.Printf("  %s★ %s reviews; removed=%s\n", mapsreview.FormatPtrFloat(row.Rating, 1), mapsreview.FormatPtrInt(row.ReviewCount), removed)
		}
		rows[row.ID] = row
		changedSinceSave++
		if err := saveIfNeeded(false); err != nil {
			return nil, err
		}
		sleepBetweenPlaces(i, len(todo), args)
	}
	if err := saveIfNeeded(true); err != nil {
		return nil, err
	}
	out := mapValues(rows)
	mapsreview.SortPlaces(out)
	return out, nil
}

func auditBannerPlaces(ctx context.Context, discoveries []mapsreview.Discovery, args args, rows map[string]mapsreview.Place) ([]mapsreview.Place, error) {
	todo := make([]mapsreview.Discovery, 0, len(discoveries))
	for _, place := range discoveries {
		row, ok := rows[place.ID]
		if ok && row.Status == "success" && !row.HasDefamationNotice {
			todo = append(todo, place)
		}
	}
	if args.ScrapeStart > 1 {
		if args.ScrapeStart > len(todo) {
			todo = nil
		} else {
			todo = todo[args.ScrapeStart-1:]
		}
	}
	if args.ScrapeLimit > 0 && args.ScrapeLimit < len(todo) {
		todo = todo[:args.ScrapeLimit]
	}
	fmt.Printf("\nBanner audit: %d no-banner rows / %d discovered", len(todo), len(discoveries))
	if args.ScrapeStart > 1 {
		fmt.Printf(" (starting at audit position %d)", args.ScrapeStart)
	}
	if args.ScrapeLimit > 0 {
		fmt.Printf(" (limit %d)", args.ScrapeLimit)
	}
	fmt.Println()

	changedSinceSave := 0
	saveEvery := max(1, args.SaveEvery)
	saveIfNeeded := func(force bool) error {
		if changedSinceSave == 0 || (!force && changedSinceSave < saveEvery) {
			return nil
		}
		if err := saveRows(args, rows); err != nil {
			return err
		}
		changedSinceSave = 0
		return nil
	}

	for i, place := range todo {
		fmt.Printf("[%d/%d] %s\n", i+1, len(todo), displayPlaceName(place))
		previousRow := rows[place.ID]
		notice, stats, _, err := extractNoticeWithAttempts(ctx, place, args.NoticeAttempts)
		if err != nil {
			fmt.Printf("  ERROR: %s; keeping existing row\n", err.Error())
			if errors.Is(err, context.Canceled) {
				if saveErr := saveIfNeeded(true); saveErr != nil {
					return nil, saveErr
				}
				return mapValues(rows), err
			}
			sleepBetweenPlaces(i, len(todo), args)
			continue
		}
		if notice == nil {
			fmt.Println("  no banner found")
			sleepBetweenPlaces(i, len(todo), args)
			continue
		}
		next := previousRow
		next.Status = "success"
		next.Error = nil
		next.ReadAt = mapsreview.NowISO()
		applyStatsIfPresent(&next, stats)
		applyNotice(&next, notice)
		mapsreview.EnrichPlaceLocation(&next)
		mapsreview.ApplyPlaceOverrides(&next)
		mapsreview.ComputeMetrics(&next)
		rows[next.ID] = next
		changedSinceSave++
		fmt.Printf("  FOUND banner: %s\n", notice.Text)
		if err := saveIfNeeded(false); err != nil {
			return nil, err
		}
		sleepBetweenPlaces(i, len(todo), args)
	}
	if err := saveIfNeeded(true); err != nil {
		return nil, err
	}
	out := mapValues(rows)
	mapsreview.SortPlaces(out)
	return out, nil
}

func preservePreviousMetadata(previous, next mapsreview.Place) mapsreview.Place {
	if next.Name == "" {
		next.Name = previous.Name
	}
	if next.Postcode == nil {
		next.Postcode = previous.Postcode
	}
	if next.Address == nil {
		next.Address = previous.Address
	}
	if next.Category == nil {
		next.Category = previous.Category
	}
	if next.Lat == nil {
		next.Lat = previous.Lat
	}
	if next.Lng == nil {
		next.Lng = previous.Lng
	}
	if next.BezirkID == nil {
		next.BezirkID = previous.BezirkID
	}
	if next.BezirkName == nil {
		next.BezirkName = previous.BezirkName
	}
	return next
}

func shouldKeepPreviousRow(previous, next mapsreview.Place, hadPrevious bool, args args) (bool, string) {
	if !hadPrevious || previous.Status != "success" || next.Status != "success" {
		return false, ""
	}
	if previous.Rating != nil && next.Rating == nil && (next.ReviewCount == nil || *next.ReviewCount > 0) {
		return true, "new scrape is missing rating"
	}
	if previous.ReviewCount != nil && next.ReviewCount == nil {
		return true, "new scrape is missing review count"
	}
	if previous.HasDefamationNotice && !next.HasDefamationNotice && !args.AllowBannerClears {
		return true, "new scrape would clear an existing deletion banner; rerun with --allow-banner-clears after manual verification"
	}
	return false, ""
}

func displayPlaceName(place mapsreview.Discovery) string {
	if place.Name != "" {
		return place.Name
	}
	return place.ID
}

func saveRows(args args, rows map[string]mapsreview.Place) error {
	out := mapValues(rows)
	for i := range out {
		mapsreview.EnrichPlaceLocation(&out[i])
		mapsreview.ApplyPlaceOverrides(&out[i])
	}
	mapsreview.SortPlaces(out)
	if err := mapsreview.WriteJSON(args.Out, out); err != nil {
		return err
	}
	return mapsreview.WritePlacesCSV(args.CSV, out)
}

func mapValues(rows map[string]mapsreview.Place) []mapsreview.Place {
	out := make([]mapsreview.Place, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi := mapsreview.StringValue(out[i].Postcode)
		pj := mapsreview.StringValue(out[j].Postcode)
		if pi != pj {
			return pi < pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sleepBetweenPlaces(index, total int, args args) {
	if index+1 >= total {
		return
	}
	sleep(randomDelay(args.DelayMin, args.DelayMax))
}

func randomDelay(min, max int) int {
	if max < min || max == 0 {
		return min
	}
	return min + rand.Intn(max-min+1)
}

func sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

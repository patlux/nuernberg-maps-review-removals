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

	"nuernberg-maps-review-removals/internal/mapsreview"
)

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

	browserCtx, cancel := mapsreview.NewBrowserContext(args.Headless)
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

type metadata struct {
	ReadAt        string   `json:"readAt"`
	Postcodes     []string `json:"postcodes"`
	Queries       []string `json:"queries"`
	MaxResults    int      `json:"maxResults"`
	Headless      bool     `json:"headless"`
	DiscoveryOnly bool     `json:"discoveryOnly"`
	ScrapeOnly    bool     `json:"scrapeOnly"`
	DelayMin      int      `json:"delayMin"`
	DelayMax      int      `json:"delayMax"`
	Output        string   `json:"output"`
	CSV           string   `json:"csv"`
	UserAgent     string   `json:"userAgent"`
	Discovered    int      `json:"discovered"`
	Rows          int      `json:"rows"`
	Success       int      `json:"success"`
	Errors        int      `json:"errors"`
}

func writeMetadata(args args, discoveries []mapsreview.Discovery, rows []mapsreview.Place) error {
	m := metadata{
		ReadAt:        mapsreview.NowISO(),
		Postcodes:     args.Postcodes,
		Queries:       args.Queries,
		MaxResults:    args.MaxResults,
		Headless:      args.Headless,
		DiscoveryOnly: args.DiscoveryOnly,
		ScrapeOnly:    args.ScrapeOnly,
		DelayMin:      args.DelayMin,
		DelayMax:      args.DelayMax,
		Output:        args.Out,
		CSV:           args.CSV,
		UserAgent:     mapsreview.UserAgent,
		Discovered:    len(discoveries),
		Rows:          len(rows),
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
	if err := navigate(ctx, mapsreview.NormalizeURL(discovery.URL), 60*time.Second); err != nil {
		return mapsreview.Place{}, err
	}
	_ = acceptConsent(ctx)
	sleep(2500)
	overview, err := readMapText(ctx)
	if err != nil {
		return mapsreview.Place{}, err
	}
	clickReviewsTab(ctx)
	reviews, err := readMapText(ctx)
	if err != nil {
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
	rawText := overview.Text + "\n" + reviews.Text
	name := rawH1
	if name == "" {
		name = discovery.Name
	}
	if name == "" {
		name = regexp.MustCompile(`(?i) - Google Maps.*`).ReplaceAllString(rawTitle, "")
		name = strings.TrimSpace(name)
	}

	stats := mapsreview.ParsePlaceStats(rawText)
	address := mapsreview.ExtractAddress(overview.Text)
	postcode := mapsreview.StringPtr(discovery.DiscoveredPostcode)
	if address != nil {
		if pc := mapsreview.ExtractPostcode(*address); pc != nil {
			postcode = pc
		}
	}
	category := extractCategory(overview.Text)
	notice := mapsreview.ParseNotice(rawText)
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
		Status:      "success",
	}
	if coords != nil {
		row.Lat = mapsreview.FloatPtr(coords.Lat)
		row.Lng = mapsreview.FloatPtr(coords.Lng)
	}
	if notice != nil {
		row.HasDefamationNotice = true
		row.RemovedMin = mapsreview.IntPtr(notice.Min)
		row.RemovedMax = notice.Max
		row.RemovedEstimate = mapsreview.FloatPtr(notice.Estimate)
		row.RemovedText = mapsreview.StringPtr(notice.Text)
	}
	mapsreview.ComputeMetrics(&row)
	return row, nil
}

func extractCategory(text string) *string {
	top := text
	if len(top) > 2500 {
		top = top[:2500]
	}
	match := regexp.MustCompile(`(?i)\n([^\n]*(?:Restaurant|Café|Cafe|Bäckerei|Imbiss|Pizzeria|Bar|Küche|Döner)[^\n]*)\n`).FindStringSubmatch(top)
	if len(match) > 1 {
		category := strings.TrimSpace(match[1])
		if category != "" {
			return mapsreview.StringPtr(category)
		}
	}
	return nil
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

	todo := make([]mapsreview.Discovery, 0, len(discoveries))
	for _, place := range discoveries {
		row, ok := rows[place.ID]
		if !ok || row.Status != "success" {
			todo = append(todo, place)
		}
	}
	fmt.Printf("\nScrape: %d remaining / %d discovered\n", len(todo), len(discoveries))

	for i, place := range todo {
		fmt.Printf("[%d/%d] %s\n", i+1, len(todo), displayPlaceName(place))
		row, err := extractPlace(ctx, place)
		if err != nil {
			errorText := err.Error()
			row = mapsreview.Place{
				ID:       place.ID,
				Name:     place.Name,
				Postcode: mapsreview.StringPtr(place.DiscoveredPostcode),
				URL:      mapsreview.NormalizeURL(place.URL),
				ReadAt:   mapsreview.NowISO(),
				Status:   "error",
				Error:    mapsreview.StringPtr(errorText),
			}
			fmt.Printf("  ERROR: %s\n", errorText)
			_ = screenshot(ctx, filepath.Join("debug", safeFilename(place.ID)+".png"))
		} else {
			removed := "none"
			if row.RemovedText != nil {
				removed = *row.RemovedText
			}
			fmt.Printf("  %s★ %s reviews; removed=%s\n", mapsreview.FormatPtrFloat(row.Rating, 1), mapsreview.FormatPtrInt(row.ReviewCount), removed)
		}
		rows[row.ID] = row
		if err := saveRows(args, rows); err != nil {
			return nil, err
		}
		sleep(randomDelay(args.DelayMin, args.DelayMax))
	}
	out := mapValues(rows)
	mapsreview.SortPlaces(out)
	return out, nil
}

func displayPlaceName(place mapsreview.Discovery) string {
	if place.Name != "" {
		return place.Name
	}
	return place.ID
}

func saveRows(args args, rows map[string]mapsreview.Place) error {
	out := mapValues(rows)
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

func randomDelay(min, max int) int {
	if max < min || max == 0 {
		return min
	}
	return min + rand.Intn(max-min+1)
}

func sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

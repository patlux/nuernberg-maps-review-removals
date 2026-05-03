package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

type args struct {
	City              string
	Postcodes         []string
	Queries           []string
	MaxResults        int
	Headless          bool
	CDPURL            string
	DiscoveryOnly     bool
	ScrapeOnly        bool
	RescrapeAll       bool
	BannerAuditOnly   bool
	AllowBannerClears bool
	NoticeAttempts    int
	ScrapeStart       int
	ScrapeLimit       int
	SaveEvery         int
	DelayMin          int
	DelayMax          int
	Out               string
	CSV               string
}

func parseArgs(argv []string) (args, error) {
	csvSet := false
	out := args{
		City:           mapsreview.DefaultCity,
		Postcodes:      mapsreview.NurembergPostcodes,
		Queries:        mapsreview.DefaultQueries,
		Headless:       false,
		DelayMin:       2500,
		DelayMax:       6000,
		SaveEvery:      1,
		NoticeAttempts: 2,
		Out:            mapsreview.ResultsJSON,
		CSV:            mapsreview.ResultsCSV,
		MaxResults:     0,
		ScrapeStart:    1,
	}

	for i := 0; i < len(argv); i++ {
		key, value, consume := mapsreview.SplitArg(argv, i)
		switch key {
		case "--city":
			out.City = value
		case "--postcodes":
			if value == "" || value == "all" {
				out.Postcodes = mapsreview.NurembergPostcodes
			} else {
				out.Postcodes = splitCSV(value)
			}
		case "--queries":
			out.Queries = splitCSV(value)
		case "--max-results":
			out.MaxResults = mapsreview.Atoi(value)
		case "--headless":
			out.Headless = mapsreview.ParseBool(value, true)
		case "--cdp-url":
			out.CDPURL = value
		case "--discovery-only":
			out.DiscoveryOnly = true
			consume = false
		case "--scrape-only":
			out.ScrapeOnly = true
			consume = false
		case "--rescrape-all", "--all":
			out.RescrapeAll = true
			consume = false
		case "--banner-audit-only":
			out.BannerAuditOnly = true
			consume = false
		case "--allow-banner-clears":
			out.AllowBannerClears = true
			consume = false
		case "--scrape-start", "--resume-from":
			out.ScrapeStart = max(1, mapsreview.Atoi(value))
		case "--scrape-limit":
			out.ScrapeLimit = max(0, mapsreview.Atoi(value))
		case "--save-every":
			out.SaveEvery = max(1, mapsreview.Atoi(value))
		case "--notice-attempts":
			out.NoticeAttempts = max(1, mapsreview.Atoi(value))
		case "--delay-min":
			out.DelayMin = mapsreview.Atoi(value)
		case "--delay-max":
			out.DelayMax = mapsreview.Atoi(value)
		case "--out":
			out.Out = value
		case "--csv":
			out.CSV = value
			csvSet = true
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		default:
			return out, fmt.Errorf("unknown argument: %s", argv[i])
		}
		if consume {
			i++
		}
	}
	if !csvSet && out.Out != "" {
		out.CSV = strings.TrimSuffix(out.Out, filepath.Ext(out.Out)) + ".csv"
	}
	return out, nil
}

func printHelp() {
	fmt.Printf(`Usage:
  go run ./cmd/scrape --postcodes all --headless=false
  go run ./cmd/scrape --postcodes 90402,90403 --queries restaurant,café,imbiss

Options:
  --city <name>             City name for discovery. Default: %s.
  --postcodes <all|csv>     PLZ list. Default: all known Nürnberg PLZ.
  --queries <csv>           Google Maps search terms. Default: %s.
  --max-results <n>         Stop after n discovered places. 0 = unlimited.
  --headless <true|false>   Chrome headless mode. Default: false; safer for consent/CAPTCHA.
  --cdp-url <ws-url>        Experimental: use an existing CDP browser instead of Chrome, e.g. Lightpanda on ws://127.0.0.1:9333.
  --discovery-only          Only create/update output/discovery.json.
  --scrape-only             Skip discovery; scrape output/discovery.json.
  --rescrape-all, --all     Re-read every discovered place, including existing success rows.
  --banner-audit-only       Scan existing no-banner success rows for missed banners; only newly found banners are written.
  --allow-banner-clears     Allow a re-scrape to remove a previously seen deletion banner. Default: keep old banner until manually verified.
  --notice-attempts <n>     Direct-reviews attempts for banner-clear verification and banner audit. Default: 2.
  --scrape-start <n>        Start scraping at 1-based position within the todo list. Default: 1.
  --resume-from <n>         Alias for --scrape-start.
  --scrape-limit <n>        Scrape at most n todo rows. 0 = unlimited.
  --save-every <n>          Persist results every n changed rows. Default: 1.
  --delay-min <ms>          Minimum delay between place pages. Default: 2500.
  --delay-max <ms>          Maximum delay between place pages. Default: 6000.
  --out <path>              Results JSON path. Default: output/places.json.
  --csv <path>              Results CSV path. Default: output/places.csv.
`, mapsreview.DefaultCity, strings.Join(mapsreview.DefaultQueries, ","))
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

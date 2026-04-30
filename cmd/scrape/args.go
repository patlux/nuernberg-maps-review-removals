package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

type args struct {
	Postcodes     []string
	Queries       []string
	MaxResults    int
	Headless      bool
	DiscoveryOnly bool
	ScrapeOnly    bool
	RescrapeAll   bool
	ScrapeStart   int
	ScrapeLimit   int
	DelayMin      int
	DelayMax      int
	Out           string
	CSV           string
}

func parseArgs(argv []string) (args, error) {
	csvSet := false
	out := args{
		Postcodes:   mapsreview.NurembergPostcodes,
		Queries:     mapsreview.DefaultQueries,
		Headless:    false,
		DelayMin:    2500,
		DelayMax:    6000,
		Out:         mapsreview.ResultsJSON,
		CSV:         mapsreview.ResultsCSV,
		MaxResults:  0,
		ScrapeStart: 1,
	}

	for i := 0; i < len(argv); i++ {
		key, value, consume := splitArg(argv, i)
		switch key {
		case "--postcodes":
			if value == "" || value == "all" {
				out.Postcodes = mapsreview.NurembergPostcodes
			} else {
				out.Postcodes = splitCSV(value)
			}
		case "--queries":
			out.Queries = splitCSV(value)
		case "--max-results":
			out.MaxResults = atoi(value)
		case "--headless":
			out.Headless = parseBool(value, true)
		case "--discovery-only":
			out.DiscoveryOnly = true
			consume = false
		case "--scrape-only":
			out.ScrapeOnly = true
			consume = false
		case "--rescrape-all", "--all":
			out.RescrapeAll = true
			consume = false
		case "--scrape-start", "--resume-from":
			out.ScrapeStart = max(1, atoi(value))
		case "--scrape-limit":
			out.ScrapeLimit = max(0, atoi(value))
		case "--delay-min":
			out.DelayMin = atoi(value)
		case "--delay-max":
			out.DelayMax = atoi(value)
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
  --postcodes <all|csv>     Nürnberg PLZ list. Default: all known Nürnberg PLZ.
  --queries <csv>           Google Maps search terms. Default: %s.
  --max-results <n>         Stop after n discovered places. 0 = unlimited.
  --headless <true|false>   Headless browser. Default: false; safer for consent/CAPTCHA.
  --discovery-only          Only create/update output/discovery.json.
  --scrape-only             Skip discovery; scrape output/discovery.json.
  --rescrape-all, --all     Re-read every discovered place, including existing success rows.
  --scrape-start <n>        Start scraping at 1-based position within the todo list. Default: 1.
  --resume-from <n>         Alias for --scrape-start.
  --scrape-limit <n>        Scrape at most n todo rows. 0 = unlimited.
  --delay-min <ms>          Minimum delay between place pages. Default: 2500.
  --delay-max <ms>          Maximum delay between place pages. Default: 6000.
  --out <path>              Results JSON path. Default: output/places.json.
  --csv <path>              Results CSV path. Default: output/places.csv.
`, strings.Join(mapsreview.DefaultQueries, ","))
}

func splitArg(argv []string, index int) (key string, value string, consume bool) {
	arg := argv[index]
	if before, after, ok := strings.Cut(arg, "="); ok {
		return before, after, false
	}
	if index+1 < len(argv) && !strings.HasPrefix(argv[index+1], "--") {
		return arg, argv[index+1], true
	}
	return arg, "", false
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

func atoi(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func parseBool(value string, missing bool) bool {
	if value == "" {
		return missing
	}
	return value == "true" || value == "1" || value == "yes"
}

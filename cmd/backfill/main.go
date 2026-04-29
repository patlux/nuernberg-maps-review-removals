package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"nuernberg-maps-review-removals/internal/mapsreview"
)

type args struct {
	Input       string
	CSV         string
	Headless    bool
	Concurrency int
	Limit       int
	OnlyMissing bool
}

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
	rows, err := mapsreview.ReadJSON(args.Input, []mapsreview.Place{})
	if err != nil {
		return err
	}
	todo := buildTodo(rows, args)
	fmt.Printf("Backfilling addresses: %d rows\n", len(todo))
	if len(todo) == 0 {
		return nil
	}

	browserCtx, cancel := mapsreview.NewBrowserContext(args.Headless)
	defer cancel()

	var mu sync.Mutex
	next := 0
	done := 0
	found := 0

	save := func() error {
		for i := range rows {
			mapsreview.EnrichPlaceLocation(&rows[i])
		}
		mapsreview.SortPlaces(rows)
		if err := mapsreview.WriteJSON(args.Input, rows); err != nil {
			return err
		}
		return mapsreview.WritePlacesCSV(args.CSV, rows)
	}

	worker := func(workerID int) error {
		tabCtx, tabCancel := chromedp.NewContext(browserCtx)
		defer tabCancel()
		for {
			mu.Lock()
			if next >= len(todo) {
				mu.Unlock()
				return nil
			}
			index := todo[next]
			next++
			mu.Unlock()

			row := &rows[index]
			address, err := readAddress(tabCtx, row.URL)
			mu.Lock()
			if err != nil {
				done++
				fmt.Printf("[%d/%d] ERROR %s: %v\n", done, len(todo), row.Name, err)
				mu.Unlock()
				continue
			}
			if address != "" {
				row.Address = mapsreview.StringPtr(address)
				if postcode := mapsreview.ExtractPostcode(address); postcode != nil {
					row.Postcode = postcode
				}
				mapsreview.EnrichPlaceLocation(row)
				found++
			}
			done++
			shouldSave := done%10 == 0 || address != ""
			if done%25 == 0 || address != "" {
				mark := "–"
				if address != "" {
					mark = "✓"
				}
				fmt.Printf("[%d/%d] %s %s", done, len(todo), mark, row.Name)
				if address != "" {
					fmt.Printf(" — %s", address)
				}
				fmt.Println()
			}
			var saveErr error
			if shouldSave {
				saveErr = save()
			}
			mu.Unlock()
			if saveErr != nil {
				return saveErr
			}
		}
	}

	workers := args.Concurrency
	if workers > len(todo) {
		workers = len(todo)
	}
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func(id int) { errs <- worker(id) }(i + 1)
	}
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			return err
		}
	}
	if err := save(); err != nil {
		return err
	}
	fmt.Printf("Done. Found %d addresses. Total with address: %d\n", found, countWithAddress(rows))
	return nil
}

func parseArgs(argv []string) (args, error) {
	out := args{Input: mapsreview.ResultsJSON, CSV: mapsreview.ResultsCSV, Headless: true, Concurrency: 4, OnlyMissing: true}
	for i := 0; i < len(argv); i++ {
		key, value, consume := splitArg(argv, i)
		switch key {
		case "--input":
			out.Input = value
		case "--csv":
			out.CSV = value
		case "--headless":
			out.Headless = parseBool(value, true)
		case "--concurrency":
			out.Concurrency = max(1, atoi(value))
		case "--limit":
			out.Limit = atoi(value)
		case "--all":
			out.OnlyMissing = false
			consume = false
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
	return out, nil
}

func printHelp() {
	fmt.Println(`Usage:
  go run ./cmd/backfill --input output/places.json --headless=true

Options:
  --input <path>          Results JSON. Default: output/places.json.
  --csv <path>            Results CSV. Default: output/places.csv.
  --headless <bool>       Run Chrome headless. Default: true.
  --concurrency <n>       Parallel tabs. Default: 4.
  --limit <n>             Stop after n rows. 0 = unlimited.
  --all                   Re-read every row, not just missing addresses.`)
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

func buildTodo(rows []mapsreview.Place, args args) []int {
	indexes := make([]int, 0, len(rows))
	for i, row := range rows {
		if row.URL == "" {
			continue
		}
		if args.OnlyMissing && row.Address != nil && *row.Address != "" {
			continue
		}
		indexes = append(indexes, i)
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		a := rows[indexes[i]]
		b := rows[indexes[j]]
		if a.HasDefamationNotice != b.HasDefamationNotice {
			return a.HasDefamationNotice
		}
		if mapsreview.RemovedSortValue(a) != mapsreview.RemovedSortValue(b) {
			return mapsreview.RemovedSortValue(a) > mapsreview.RemovedSortValue(b)
		}
		return a.Name < b.Name
	})
	if args.Limit > 0 && len(indexes) > args.Limit {
		indexes = indexes[:args.Limit]
	}
	return indexes
}

func readAddress(ctx context.Context, rawURL string) (string, error) {
	if err := mapsreview.RunWithTimeout(ctx, 25*time.Second,
		chromedp.Navigate(mapsreview.NormalizeURL(rawURL)),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return "", err
	}
	_ = acceptConsent(ctx)
	for i := 0; i < 14; i++ {
		if i == 0 {
			sleep(1800)
		} else {
			sleep(500)
		}
		var address string
		err := mapsreview.RunWithTimeout(ctx, 5*time.Second, chromedp.Evaluate(`(() => {
  const attrTexts = Array.from(document.querySelectorAll('[aria-label], [alt], [data-tooltip]'))
    .flatMap(el => [el.getAttribute('aria-label'), el.getAttribute('alt'), el.getAttribute('data-tooltip')])
    .filter(Boolean);
  const text = [document.body.innerText, ...attrTexts].join('\n');
  const match = text.match(/Adresse:\s*([^\n]*\b9\d{4}\s+[^\n]*)/i) || text.match(/\n([^\n]*,\s*9\d{4}\s+[^\n]*)\n/i);
  return match?.[1]?.replace(/^Adresse:\s*/i, '').trim() || '';
})()`, &address))
		if err != nil {
			return "", err
		}
		if address != "" {
			return address, nil
		}
	}
	return "", nil
}

func acceptConsent(ctx context.Context) error {
	var accepted bool
	err := mapsreview.RunWithTimeout(ctx, 5*time.Second, chromedp.Evaluate(`(() => {
  const patterns = [/Alle akzeptieren/i, /Accept all/i, /Ich stimme zu/i, /Akzeptieren/i];
  const buttons = Array.from(document.querySelectorAll('button, [role="button"]'));
  const button = buttons.find(el => patterns.some(pattern => pattern.test(el.innerText || el.textContent || el.getAttribute('aria-label') || '')));
  if (!button) return false;
  button.click();
  return true;
})()`, &accepted))
	if accepted {
		sleep(1000)
	}
	return err
}

func countWithAddress(rows []mapsreview.Place) int {
	count := 0
	for _, row := range rows {
		if row.Address != nil && *row.Address != "" {
			count++
		}
	}
	return count
}

func sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

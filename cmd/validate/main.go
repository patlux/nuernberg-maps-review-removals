package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

type args struct {
	Input           string
	StrictNuremberg bool
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
	valid := mapsreview.ValidRows(rows)
	fatal := []string{}
	warnings := []string{}

	missingAddress := 0
	missingStats := 0
	missingCoords := 0
	missingBezirk := 0
	statusErrors := 0
	outside := map[string]int{}
	ids := map[string]int{}
	urls := map[string]int{}
	bannerParseProblems := 0

	for _, row := range rows {
		if row.ID == "" {
			fatal = append(fatal, "row with missing id")
		} else {
			ids[row.ID]++
		}
		if row.URL == "" {
			fatal = append(fatal, fmt.Sprintf("%s: missing url", row.Name))
		} else {
			urls[row.URL]++
		}
		if row.Status != "success" {
			statusErrors++
		}
		if row.Status == "success" {
			if row.Address == nil || *row.Address == "" {
				missingAddress++
			}
			if row.Rating == nil || row.ReviewCount == nil {
				missingStats++
			}
			if (row.Lat == nil || row.Lng == nil) && mapsreview.ExtractCoordinates(row.URL) == nil {
				missingCoords++
			}
			if mapsreview.NurembergPostcodeSet[mapsreview.StringValue(row.Postcode)] && (row.BezirkID == nil || row.BezirkName == nil) {
				missingBezirk++
			}
		}
		if postcode := mapsreview.StringValue(row.Postcode); postcode != "" && !mapsreview.NurembergPostcodeSet[postcode] {
			outside[postcode]++
		}
		if row.HasDefamationNotice && (row.RemovedMin == nil || row.RemovedEstimate == nil || row.RemovedText == nil) {
			bannerParseProblems++
		}
	}

	for id, count := range ids {
		if count > 1 {
			fatal = append(fatal, fmt.Sprintf("duplicate id %q (%d rows)", id, count))
		}
	}
	for url, count := range urls {
		if count > 1 {
			warnings = append(warnings, fmt.Sprintf("duplicate url %q (%d rows)", url, count))
		}
	}
	if statusErrors > 0 {
		warnings = append(warnings, fmt.Sprintf("%d rows have non-success status", statusErrors))
	}
	if missingAddress > 0 {
		warnings = append(warnings, fmt.Sprintf("%d success rows are missing addresses", missingAddress))
	}
	if missingStats > 0 {
		warnings = append(warnings, fmt.Sprintf("%d success rows are missing rating or review count", missingStats))
	}
	if missingCoords > 0 {
		warnings = append(warnings, fmt.Sprintf("%d success rows are missing coordinates", missingCoords))
	}
	if missingBezirk > 0 {
		warnings = append(warnings, fmt.Sprintf("%d Nürnberg success rows are missing statistical district assignment", missingBezirk))
	}
	if len(outside) > 0 {
		parts := make([]string, 0, len(outside))
		for postcode, count := range outside {
			parts = append(parts, fmt.Sprintf("%s:%d", postcode, count))
		}
		sort.Strings(parts)
		message := "outside Nürnberg PLZ: " + strings.Join(parts, ", ")
		if args.StrictNuremberg {
			fatal = append(fatal, message)
		} else {
			warnings = append(warnings, message)
		}
	}
	if bannerParseProblems > 0 {
		fatal = append(fatal, fmt.Sprintf("%d banner rows are missing parsed removal fields", bannerParseProblems))
	}

	fmt.Printf("Input: %s\n", args.Input)
	fmt.Printf("Rows: %s total, %s valid, %s banners\n", mapsreview.FormatGermanInt(len(rows)), mapsreview.FormatGermanInt(len(valid)), mapsreview.FormatGermanInt(len(mapsreview.RemovedRows(rows))))
	if len(warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, warning := range warnings {
			fmt.Println("  - " + warning)
		}
	}
	if len(fatal) > 0 {
		fmt.Println("\nErrors:")
		for _, item := range fatal {
			fmt.Println("  - " + item)
		}
		return fmt.Errorf("validation failed with %d errors", len(fatal))
	}
	fmt.Println("\nValidation passed.")
	return nil
}

func parseArgs(argv []string) (args, error) {
	out := args{Input: mapsreview.ResultsJSON}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		key, value, consume := splitArg(argv, i)
		switch key {
		case "--input":
			out.Input = value
		case "--strict-nuremberg":
			out.StrictNuremberg = true
			consume = false
		case "--help", "-h":
			fmt.Println(`Usage:
  go run ./cmd/validate --input output/places.json

Options:
  --input <path>          Results JSON. Default: output/places.json.
  --strict-nuremberg     Treat outside-Nürnberg postcodes as errors.`)
			os.Exit(0)
		default:
			return out, fmt.Errorf("unknown argument: %s", arg)
		}
		if consume {
			i++
		}
	}
	return out, nil
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

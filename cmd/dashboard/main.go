package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

//go:embed app.js
var dashboardJS string

//go:embed analytics.js
var analyticsJS string

//go:embed vendor/leaflet/leaflet.js
var leafletJS string

//go:embed vendor/leaflet/leaflet.css
var leafletCSS string

//go:embed vendor/leaflet/LICENSE
var leafletLicense string

const (
	defaultInput      = mapsreview.ResultsJSON
	defaultOutput     = "output/charts/nuernberg_dashboard.html"
	defaultConfigPath = "config.toml"
)

type args struct {
	City                 string
	Input                string
	Output               string
	LegalName            string
	LegalEmail           string
	LegalAddressLines    []string
	LegalNote            string
	LegalPostHandler     string
	AnalyticsSrc         string
	AnalyticsDomain      string
	SiteDomain           string
	SiteURL              string
	SiteOutput           string
	BuildSite            bool
	CartoAPIKey          string
	CartoEnabled         bool
	CartoPrivacyText     string
	AnalyticsEnabled     bool
	AnalyticsPrivacyText string
}

type dashboardConfig struct {
	City       string
	Input      string
	Output     string
	SiteDomain string
	SiteURL    string
	SiteOutput string
	Legal      struct {
		Enabled      *bool
		Name         string
		Email        string
		AddressLines []string
		Note         string
		PostHandler  string
	}
	Analytics struct {
		Src         string
		Domain      string
		Enabled     bool
		PrivacyText string
	}
	PlacesAPI struct {
		APIKey string
	}
	Carto struct {
		APIKey      string
		Enabled     bool
		PrivacyText string
	}
}

type clientRow struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Postcode         string   `json:"postcode"`
	Lat              *float64 `json:"lat,omitempty"`
	Lng              *float64 `json:"lng,omitempty"`
	BezirkID         string   `json:"bezirkId"`
	BezirkName       string   `json:"bezirkName"`
	BezirkLabel      string   `json:"bezirkLabel"`
	Rating           *float64 `json:"rating"`
	ReviewCount      *int     `json:"reviewCount"`
	Category         string   `json:"category"`
	ParentCategory   string   `json:"parentCategory"`
	HasBanner        bool     `json:"hasBanner"`
	RemovedRange     string   `json:"removedRange"`
	RemovedMin       *int     `json:"removedMin"`
	RemovedMax       *int     `json:"removedMax"`
	RemovedEstimate  float64  `json:"removedEstimate"`
	DeletionRatioPct *float64 `json:"deletionRatioPct"`
	URL              string   `json:"url"`
	Address          string   `json:"address"`
	ReadAt           string   `json:"readAt"`
	PlaceState       string   `json:"placeState,omitempty"`
}

type seoStats struct {
	Total           int
	Banners         int
	Clean           int
	RemovedEstimate int
	Snapshot        string
	Coverage        string
	Top             []clientRow
}

type dashboardMetadata struct {
	SiteName        string
	PageTitle       string
	PageDescription string
	CanonicalURL    string
}

func dashboardMeta(args args, city string) dashboardMetadata {
	siteURL := normalizedSiteURL(args.SiteURL)
	return dashboardMetadata{
		SiteName:        fmt.Sprintf("%s Maps Review Removals", city),
		PageTitle:       fmt.Sprintf("%s Google-Maps-Bewertungen: Dashboard zu Google-Hinweisen", city),
		PageDescription: fmt.Sprintf("Historische Momentaufnahme öffentlich beobachteter Google-Maps-Hinweise im Raum %s: Spannen, Näherungswerte, Karte und Daten-Explorer.", city),
		CanonicalURL:    siteURL,
	}
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
	if strings.TrimSpace(args.CartoAPIKey) == "" {
		return fmt.Errorf("carto.api_key is required: configure a CARTO Basemaps key in config.toml before building the dashboard")
	}
	rows, err := mapsreview.ReadJSON(args.Input, []mapsreview.Place{})
	if err != nil {
		return err
	}
	data := makeClientRows(rows)
	if err := os.MkdirAll(filepath.Dir(args.Output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(args.Output, []byte(makeHTML(args, data)), 0o644); err != nil {
		return err
	}
	if err := syncLegalPages(args, filepath.Dir(args.Output)); err != nil {
		return err
	}
	if args.BuildSite {
		if err := buildSite(args); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %s\n", args.Output)
	return nil
}

func parseArgs(argv []string) (args, error) {
	out := defaultArgs()
	configPath, err := findConfigPath(argv)
	if err != nil {
		return out, err
	}
	if configPath != "" {
		out, err = readDashboardConfig(configPath, out)
		if err != nil {
			return out, err
		}
	}

	cliLegalAddressSeen := false
	for i := 0; i < len(argv); i++ {
		key, value, consume := splitArg(argv, i)
		switch key {
		case "--config":
			// Loaded before CLI flags so explicit flags can override config values.
		case "--city":
			out.City = value
		case "--input":
			out.Input = value
		case "--output":
			out.Output = value
		case "--legal-name":
			out.LegalName = value
		case "--legal-email":
			out.LegalEmail = value
		case "--legal-address-line":
			if !cliLegalAddressSeen {
				out.LegalAddressLines = nil
				cliLegalAddressSeen = true
			}
			out.LegalAddressLines = append(out.LegalAddressLines, value)
		case "--legal-note":
			out.LegalNote = value
		case "--legal-post-handler":
			out.LegalPostHandler = value
		case "--analytics-src":
			out.AnalyticsSrc = value
		case "--analytics-domain":
			out.AnalyticsDomain = value
		case "--site":
			out.BuildSite = true
			consume = false
		case "--site-output":
			out.SiteOutput = value
		case "--site-domain":
			out.SiteDomain = value
		case "--site-url":
			out.SiteURL = value
		case "--help", "-h":
			fmt.Printf(`Usage:
  go run ./cmd/dashboard
  go run ./cmd/dashboard --config config.toml
  go run ./cmd/dashboard --input output/places.json --output output/charts/nuernberg_dashboard.html

Options:
  --config <path>                       TOML config file. Defaults to config.toml when present. CLI flags override config values.
  --city <name>                         Displayed city name. Default: %s.
  --input <path>                        Scrape results JSON. Default: %s.
  --output <path>                       Dashboard HTML path. Default: %s.
  --legal-name <name>                   Generate legal pages with this responsible person.
  --legal-email <email>                 Email address for legal pages.
  --legal-address-line <line>           Address line for legal pages. Repeat for multiple lines.
  --legal-note <text>                   Optional note, e.g. c/o address clarification.
  --legal-post-handler <name>           Optional entity handling postal delivery.
  --analytics-src <url>                 Optional Plausible script URL.
  --analytics-domain <domain>           Optional Plausible data-domain.
  --site                                Build public/ publishing artifact from generated dashboard outputs.
  --site-output <path>                  Publishing artifact directory. Default from config.
  --site-domain <domain>                Domain for CNAME, from config.toml.
  --site-url <url>                      Public canonical URL, from config.toml.
`, mapsreview.DefaultCity, defaultInput, defaultOutput)
			os.Exit(0)
		default:
			return out, fmt.Errorf("unknown argument: %s", argv[i])
		}
		if consume {
			i++
		}
	}
	out.City = strings.TrimSpace(out.City)
	if out.City == "" {
		return out, fmt.Errorf("--city must not be empty")
	}
	if out.CartoEnabled && (out.CartoPrivacyText == "" || !legalPagesEnabled(out)) {
		return out, fmt.Errorf("enabled CARTO tiles require legal pages and verified carto.privacy_text")
	}
	if out.AnalyticsEnabled && (out.AnalyticsPrivacyText == "" || out.AnalyticsSrc == "" || !legalPagesEnabled(out)) {
		return out, fmt.Errorf("enabled analytics require legal pages, analytics.src and verified analytics.privacy_text")
	}
	if err := validateLegalArgs(out); err != nil {
		return out, err
	}
	return out, nil
}

func defaultArgs() args {
	return args{City: mapsreview.DefaultCity, Input: defaultInput, Output: defaultOutput, SiteOutput: "public"}
}

func findConfigPath(argv []string) (string, error) {
	for i := 0; i < len(argv); i++ {
		key, value, consume := splitArg(argv, i)
		if key == "--config" {
			if strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("--config requires a path")
			}
			return value, nil
		}
		if consume {
			i++
		}
	}
	if _, err := os.Stat(defaultConfigPath); err == nil {
		return defaultConfigPath, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "", nil
}

func readDashboardConfig(path string, base args) (args, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return base, err
	}
	cfg, err := parseDashboardTOML(string(data))
	if err != nil {
		return base, fmt.Errorf("read dashboard config %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.City) != "" {
		base.City = cfg.City
	}
	if strings.TrimSpace(cfg.Input) != "" {
		base.Input = cfg.Input
	}
	if strings.TrimSpace(cfg.Output) != "" {
		base.Output = cfg.Output
	}
	if strings.TrimSpace(cfg.SiteDomain) != "" {
		base.SiteDomain = cfg.SiteDomain
	}
	if strings.TrimSpace(cfg.SiteURL) != "" {
		base.SiteURL = cfg.SiteURL
	}
	if strings.TrimSpace(cfg.SiteOutput) != "" {
		base.SiteOutput = cfg.SiteOutput
	}
	if cfg.Legal.Enabled != nil && !*cfg.Legal.Enabled {
		base.LegalName = ""
		base.LegalEmail = ""
		base.LegalAddressLines = nil
		base.LegalNote = ""
		base.LegalPostHandler = ""
	} else {
		if strings.TrimSpace(cfg.Legal.Name) != "" {
			base.LegalName = cfg.Legal.Name
		}
		if strings.TrimSpace(cfg.Legal.Email) != "" {
			base.LegalEmail = cfg.Legal.Email
		}
		if len(cfg.Legal.AddressLines) > 0 {
			base.LegalAddressLines = cfg.Legal.AddressLines
		}
		if strings.TrimSpace(cfg.Legal.Note) != "" {
			base.LegalNote = cfg.Legal.Note
		}
		if strings.TrimSpace(cfg.Legal.PostHandler) != "" {
			base.LegalPostHandler = cfg.Legal.PostHandler
		}
	}
	if strings.TrimSpace(cfg.Analytics.Src) != "" {
		base.AnalyticsSrc = cfg.Analytics.Src
	}
	if strings.TrimSpace(cfg.Analytics.Domain) != "" {
		base.AnalyticsDomain = cfg.Analytics.Domain
	}
	base.CartoAPIKey = strings.TrimSpace(cfg.Carto.APIKey)
	base.CartoEnabled = cfg.Carto.Enabled
	base.CartoPrivacyText = strings.TrimSpace(cfg.Carto.PrivacyText)
	base.AnalyticsEnabled = cfg.Analytics.Enabled
	base.AnalyticsPrivacyText = strings.TrimSpace(cfg.Analytics.PrivacyText)
	return base, nil
}

func parseDashboardTOML(input string) (dashboardConfig, error) {
	var cfg dashboardConfig
	section := ""
	lines := strings.Split(input, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripTomlComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return cfg, fmt.Errorf("invalid TOML line %d", i+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "[") && !strings.Contains(value, "]") {
			for i+1 < len(lines) {
				i++
				next := strings.TrimSpace(stripTomlComment(lines[i]))
				value += " " + next
				if strings.Contains(next, "]") {
					break
				}
			}
		}
		if err := setDashboardConfigValue(&cfg, section, key, value); err != nil {
			return cfg, fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return cfg, nil
}

func stripTomlComment(line string) string {
	inString := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if r == '#' && !inString {
			return line[:i]
		}
	}
	return line
}

func setDashboardConfigValue(cfg *dashboardConfig, section string, key string, value string) error {
	switch section {
	case "":
		s, err := parseTomlString(value)
		if err != nil {
			return err
		}
		switch key {
		case "city":
			cfg.City = s
		case "input":
			cfg.Input = s
		case "output":
			cfg.Output = s
		case "site_domain":
			cfg.SiteDomain = s
		case "site_url":
			cfg.SiteURL = s
		case "site_output":
			cfg.SiteOutput = s
		default:
			return fmt.Errorf("unknown config key %q", key)
		}
	case "legal":
		switch key {
		case "enabled":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean for legal.enabled")
			}
			cfg.Legal.Enabled = &v
		case "name":
			v, err := parseTomlString(value)
			if err != nil {
				return err
			}
			cfg.Legal.Name = v
		case "email":
			v, err := parseTomlString(value)
			if err != nil {
				return err
			}
			cfg.Legal.Email = v
		case "address_lines", "addressLines":
			v, err := parseTomlStringArray(value)
			if err != nil {
				return err
			}
			cfg.Legal.AddressLines = v
		case "note":
			v, err := parseTomlString(value)
			if err != nil {
				return err
			}
			cfg.Legal.Note = v
		case "post_handler", "postHandler":
			v, err := parseTomlString(value)
			if err != nil {
				return err
			}
			cfg.Legal.PostHandler = v
		default:
			return fmt.Errorf("unknown legal config key %q", key)
		}
	case "analytics":
		if key == "enabled" {
			v, err := strconv.ParseBool(value)
			cfg.Analytics.Enabled = v
			return err
		}
		v, err := parseTomlString(value)
		if err != nil {
			return err
		}
		switch key {
		case "src":
			cfg.Analytics.Src = v
		case "domain":
			cfg.Analytics.Domain = v
		case "privacy_text":
			cfg.Analytics.PrivacyText = v
		default:
			return fmt.Errorf("unknown analytics config key %q", key)
		}
	case "carto":
		if key == "enabled" {
			v, err := strconv.ParseBool(value)
			cfg.Carto.Enabled = v
			return err
		}
		if key == "privacy_text" {
			v, err := parseTomlString(value)
			cfg.Carto.PrivacyText = v
			return err
		}
		if key != "api_key" {
			return fmt.Errorf("unknown carto config key %q", key)
		}
		v, err := parseTomlString(value)
		if err != nil {
			return fmt.Errorf("invalid quoted string for carto.api_key")
		}
		cfg.Carto.APIKey = v
	case "places_api", "placesAPI":
		v, err := parseTomlString(value)
		if err != nil {
			return err
		}
		switch key {
		case "api_key", "apiKey":
			cfg.PlacesAPI.APIKey = v
		default:
			return fmt.Errorf("unknown places_api config key %q", key)
		}
	default:
		return fmt.Errorf("unknown config section %q", section)
	}
	return nil
}

func parseTomlString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "\"") {
		return "", fmt.Errorf("expected quoted string")
	}
	out, err := strconv.Unquote(value)
	if err != nil {
		return "", err
	}
	return out, nil
}

func parseTomlStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("expected string array")
	}
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if value == "" {
		return nil, nil
	}
	parts := []string{}
	start := -1
	escaped := false
	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && start >= 0 {
			escaped = true
			continue
		}
		if r == '"' {
			if start < 0 {
				start = i
			} else {
				item, err := strconv.Unquote(value[start : i+1])
				if err != nil {
					return nil, err
				}
				parts = append(parts, item)
				start = -1
			}
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("unterminated string in array")
	}
	return parts, nil
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

func makeClientRows(rows []mapsreview.Place) []clientRow {
	out := make([]clientRow, 0, len(rows))
	for _, row := range rows {
		if row.Status != "success" || row.Name == "" || row.Rating == nil || row.ReviewCount == nil {
			continue
		}
		mapsreview.EnrichPlaceLocation(&row)
		mapsreview.ApplyPlaceOverrides(&row)
		removedEstimate := 0.0
		if row.HasDefamationNotice {
			removedEstimate = mapsreview.RemovedSortValue(row)
		}
		lat := row.Lat
		lng := row.Lng
		bezirkID := mapsreview.StringValue(row.BezirkID)
		bezirkName := mapsreview.StringValue(row.BezirkName)
		bezirkLabel := ""
		if bezirkID != "" && bezirkName != "" {
			bezirkLabel = bezirkID + " " + bezirkName
		}
		out = append(out, clientRow{
			ID:               row.ID,
			Name:             row.Name,
			Postcode:         mapsreview.StringValue(row.Postcode),
			Lat:              lat,
			Lng:              lng,
			BezirkID:         bezirkID,
			BezirkName:       bezirkName,
			BezirkLabel:      bezirkLabel,
			Rating:           row.Rating,
			ReviewCount:      row.ReviewCount,
			Category:         mapsreview.StringValue(row.Category),
			ParentCategory:   mapsreview.StringValue(row.ParentCategory),
			HasBanner:        row.HasDefamationNotice,
			RemovedRange:     mapsreview.RemovedRange(row),
			RemovedMin:       row.RemovedMin,
			RemovedMax:       row.RemovedMax,
			RemovedEstimate:  removedEstimate,
			DeletionRatioPct: row.DeletionRatioPct,
			URL:              row.URL,
			Address:          mapsreview.StringValue(row.Address),
			ReadAt:           row.ReadAt,
			PlaceState:       row.PlaceState,
		})
	}
	return out
}

func makeHTML(args args, data []clientRow) string {
	city := strings.TrimSpace(args.City)
	if city == "" {
		city = mapsreview.DefaultCity
	}
	postcodes := uniqueSorted(data, func(row clientRow) string { return row.Postcode })
	bezirke := []string{}
	bezirkBoundaries := []mapsreview.BezirkBoundary{}
	if mapsreview.IsDefaultCity(city) {
		bezirke = allBezirkLabels()
		bezirkBoundaries = mapsreview.BezirkBoundaries()
	}
	if len(bezirke) == 0 {
		bezirke = uniqueSorted(data, func(row clientRow) string { return row.BezirkLabel })
	}
	ranges := uniqueSorted(data, func(row clientRow) string { return row.RemovedRange })
	sort.SliceStable(ranges, func(i, j int) bool {
		return maxEstimateForRange(data, ranges[i]) > maxEstimateForRange(data, ranges[j])
	})
	snapshot := snapshotTime(data)
	snapshotDisplay := "unbekannt"
	if !snapshot.IsZero() {
		snapshotDisplay = snapshot.Format("02.01.2006")
	}
	stats := makeSEOStats(data, snapshotDisplay)
	meta := dashboardMeta(args, city)
	structuredData := structuredDataJSON(stats, snapshot, city, meta)

	jsonText := compactClientDataJSON(data, city)
	bezirkText := compactBezirkDataJSON(bezirkBoundaries)
	// A domain-restricted CARTO Basemaps key is necessarily visible to browsers.
	// Only export this publishable key, never the separate Google Places API key.
	configText := safeJSON(map[string]string{"city": city, "cartoApiKey": args.CartoAPIKey, "cartoEnabled": strconv.FormatBool(cartoTilesEnabled(args))})

	postcodeOptions := ""
	for _, postcode := range postcodes {
		postcodeOptions += fmt.Sprintf(`<option value="%s">%s</option>`, escAttr(postcode), esc(postcode))
	}
	bezirkOptions := ""
	if countRows(data, func(row clientRow) bool { return row.BezirkLabel == "" }) > 0 {
		bezirkOptions += `<option value="__none__">Ohne Bezirk</option>`
	}
	for _, bezirk := range bezirke {
		bezirkOptions += fmt.Sprintf(`<option value="%s">%s</option>`, escAttr(bezirk), esc(bezirk))
	}
	rangeOptions := ""
	for _, r := range ranges {
		if r != "" {
			rangeOptions += fmt.Sprintf(`<option value="%s">%s</option>`, escAttr(r), esc(r))
		}
	}
	// Build hierarchical options: parent categories first (selectable), then their sub-categories
	type catEntry struct{ parent, name string }
	parentMap := map[string][]string{}
	for _, row := range data {
		if row.Category != "" && row.ParentCategory != "" {
			parentMap[row.ParentCategory] = append(parentMap[row.ParentCategory], row.Category)
		}
	}
	// Sort parents by total count desc, then alphabetically
	parentOrder := make([]string, 0, len(parentMap))
	for parent := range parentMap {
		parentOrder = append(parentOrder, parent)
	}
	sort.SliceStable(parentOrder, func(i, j int) bool {
		ci, cj := 0, 0
		for _, row := range data {
			if row.ParentCategory == parentOrder[i] {
				ci++
			}
			if row.ParentCategory == parentOrder[j] {
				cj++
			}
		}
		if ci != cj {
			return ci > cj
		}
		return parentOrder[i] < parentOrder[j]
	})
	categoryOptions := ""
	for _, parent := range parentOrder {
		// Parent category as selectable option
		categoryOptions += fmt.Sprintf(`<option value="parent:%s">%s</option>`, escAttr(parent), esc(parent))
		// Deduplicate children
		seen := map[string]bool{}
		for _, child := range parentMap[parent] {
			if !seen[child] {
				seen[child] = true
				categoryOptions += fmt.Sprintf(`<option value="%s">&nbsp;&nbsp;%s</option>`, escAttr(child), esc(child))
			}
		}
	}

	page := `<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>__PAGE_TITLE__</title>
  <meta name="description" content="__PAGE_DESCRIPTION__">
  <meta name="robots" content="index,follow,max-image-preview:large">
  <meta name="author" content="Patrick Wozniak">
  <meta name="theme-color" content="#ffffff" media="(prefers-color-scheme: light)">
  <meta name="theme-color" content="#0e0c0b" media="(prefers-color-scheme: dark)">
  <link rel="canonical" href="__CANONICAL_URL__">
  <link rel="alternate" hreflang="de" href="__CANONICAL_URL__">
  <link rel="alternate" hreflang="x-default" href="__CANONICAL_URL__">
  <meta property="og:type" content="website">
  <meta property="og:locale" content="de_DE">
  <meta property="og:site_name" content="__SITE_NAME__">
  <meta property="og:title" content="__PAGE_TITLE__">
  <meta property="og:description" content="__PAGE_DESCRIPTION__">
  <meta property="og:url" content="__CANONICAL_URL__">
  <meta name="twitter:card" content="summary">
  <meta name="twitter:title" content="__PAGE_TITLE__">
  <meta name="twitter:description" content="__PAGE_DESCRIPTION__">
  <script type="application/ld+json">
__STRUCTURED_DATA__
  </script>
__ANALYTICS__
  <style>__LEAFLET_CSS__</style>
  <script>
    (function () {
      try {
        const savedTheme = localStorage.getItem('dashboardTheme');
        if (savedTheme === 'dark' || savedTheme === 'light') document.documentElement.dataset.theme = savedTheme;
      } catch (_) {}
    }());
  </script>
  <style>
    :root {
      color-scheme: light;
      --red: #cf2a1b;
      --red-dark: #9f2017;
      --text: #3f3f3f;
      --heading: #333;
      --muted: #6f6f6f;
      --line: #d6d6d6;
      --soft: #f4f4f4;
      --blue: #1f6f8b;
      --green: #2d7b3f;
      --orange: #d97a1d;
      --bg: #fff;
      --surface: #fff;
      --surface-raised: #fff;
      --surface-muted: #f6f6f6;
      --surface-subtle: #fafafa;
      --control-bg: #333;
      --control-text: #fff;
      --control-muted: rgba(255,255,255,.78);
      --input-bg: #fff;
      --input-text: #333;
      --nav-text: #666;
      --hero-bg: linear-gradient(180deg, rgba(38,120,155,.25), rgba(38,120,155,.25)), linear-gradient(135deg, #b7d5e1 0%, #e1edf2 45%, #bdc8cf 100%);
      --hero-title-bg: rgba(55,55,55,.78);
      --track-bg: #e7e7e7;
      --table-head-bg: #f3f3f3;
      --table-head-hover: #ececec;
      --row-hover: #fff4f2;
      --target-row: #fff0cc;
      --pill-bg: #eef0f3;
      --pill-bad-bg: #e2e8f1;
      --marker-clean: #8a8e94;
      --marker-mid: #5b7b8e;
      --marker-high: #3c4e6b;
      --pill-text: #555a62;
      --pill-bad-text: #2f3e57;
      --map-bg: #f4f4f4;
      --hint-bg: rgba(51,51,51,.88);
      --focus-blue: rgba(31,111,139,.13);
      --focus-red: rgba(207,42,27,.35);
      --shadow: 0 2px 10px rgba(0,0,0,.18);
      --sans: Arial, Helvetica, sans-serif;
    }
    @media (prefers-color-scheme: dark) {
      :root:not([data-theme="light"]) {
        color-scheme: dark;
        --red: #ff5b49;
        --red-dark: #e23c2b;
        --text: #e7e2dc;
        --heading: #fff5ec;
        --muted: #b8afa6;
        --line: #332e2b;
        --soft: #171412;
        --blue: #6ab8d8;
        --green: #72c382;
        --orange: #ff9d42;
        --bg: #0e0c0b;
        --surface: #171412;
        --surface-raised: #211c19;
        --surface-muted: #211c19;
        --surface-subtle: #141110;
        --control-bg: #fff5ec;
        --control-text: #171412;
        --control-muted: rgba(23,20,18,.68);
        --input-bg: #120f0e;
        --input-text: #fff5ec;
        --nav-text: #d4c8bd;
        --hero-bg: radial-gradient(circle at 18% 18%, rgba(255,91,73,.18), transparent 34%), linear-gradient(180deg, rgba(14,12,11,.32), rgba(14,12,11,.68)), linear-gradient(135deg, #102b35 0%, #171412 48%, #3b1c18 100%);
        --hero-title-bg: rgba(14,12,11,.82);
        --track-bg: #302925;
        --table-head-bg: #211c19;
        --table-head-hover: #2b2420;
        --row-hover: #2a1714;
        --target-row: #382b12;
        --pill-bg: rgba(180,188,200,.16);
        --pill-bad-bg: rgba(120,148,184,.20);
        --marker-clean: #9aa0a8;
        --marker-mid: #7aa3ba;
        --marker-high: #6a82af;
        --pill-text: #b8bdc6;
        --pill-bad-text: #b4cae4;
        --map-bg: #151210;
        --hint-bg: rgba(14,12,11,.92);
        --focus-blue: rgba(106,184,216,.22);
        --focus-red: rgba(255,91,73,.32);
        --shadow: 0 18px 40px rgba(0,0,0,.42);
      }
    }
    :root[data-theme="dark"] {
      color-scheme: dark;
      --red: #ff5b49;
      --red-dark: #e23c2b;
      --text: #e7e2dc;
      --heading: #fff5ec;
      --muted: #b8afa6;
      --line: #332e2b;
      --soft: #171412;
      --blue: #6ab8d8;
      --green: #72c382;
      --orange: #ff9d42;
      --bg: #0e0c0b;
      --surface: #171412;
      --surface-raised: #211c19;
      --surface-muted: #211c19;
      --surface-subtle: #141110;
      --control-bg: #fff5ec;
      --control-text: #171412;
      --control-muted: rgba(23,20,18,.68);
      --input-bg: #120f0e;
      --input-text: #fff5ec;
      --nav-text: #d4c8bd;
      --hero-bg: radial-gradient(circle at 18% 18%, rgba(255,91,73,.18), transparent 34%), linear-gradient(180deg, rgba(14,12,11,.32), rgba(14,12,11,.68)), linear-gradient(135deg, #102b35 0%, #171412 48%, #3b1c18 100%);
      --hero-title-bg: rgba(14,12,11,.82);
      --track-bg: #302925;
      --table-head-bg: #211c19;
      --table-head-hover: #2b2420;
      --row-hover: #2a1714;
      --target-row: #382b12;
      --pill-bg: rgba(114,195,130,.16);
      --pill-bad-bg: rgba(255,91,73,.16);
      --map-bg: #151210;
      --hint-bg: rgba(14,12,11,.92);
      --focus-blue: rgba(106,184,216,.22);
      --focus-red: rgba(255,91,73,.32);
      --shadow: 0 18px 40px rgba(0,0,0,.42);
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body { margin: 0; min-height: 100vh; font-family: var(--sans); color: var(--text); background: var(--bg); }
    .sitebar { height: 76px; background: var(--surface-raised); border-bottom: 1px solid var(--line); box-shadow: var(--shadow); }
    .sitebar-inner { position: relative; width: min(1320px, calc(100vw - 32px)); height: 100%; margin: 0 auto; display: flex; align-items: center; gap: 42px; }
    .top-icons { display: flex; gap: 28px; color: var(--nav-text); font-size: 33px; line-height: 1; }
    .theme-toggle { display: inline-flex; align-items: center; gap: 8px; height: 42px; margin-left: auto; margin-right: 190px; padding: 0 14px; border: 1px solid var(--line); border-radius: 999px; background: var(--surface); color: var(--heading); box-shadow: 0 1px 4px rgba(0,0,0,.12); font-weight: 700; cursor: pointer; }
    .theme-toggle:hover, .theme-toggle:focus-visible { border-color: var(--red); color: var(--red); outline: none; }
    .theme-toggle-icon { width: 1.1em; text-align: center; }
    .n-logo { position: absolute; right: 0; top: 0; width: 170px; height: 128px; padding: 66px 14px 10px; background: var(--red); color: #fff; font-size: 24px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; z-index: 5; }
    .n-logo::before { content: ""; position: absolute; left: 14px; right: 14px; top: 58px; height: 2px; background: #fff; opacity: .9; }
    .n-logo::after { content: "⌂⌂"; position: absolute; right: 13px; top: 20px; color: #fff; font-size: 36px; letter-spacing: -12px; transform: scaleX(1.4); }
    .hero { min-height: 380px; margin: 0 0 30px; background: var(--hero-bg); display: flex; align-items: end; }
    .hero-inner { width: min(1320px, calc(100vw - 32px)); margin: 0 auto; padding: 120px 0 42px; display: grid; grid-template-columns: minmax(0, 1fr) minmax(320px, 440px); gap: 22px; align-items: end; }
    .hero-title { width: min(760px, 100%); margin: 0; padding: 24px 28px; background: var(--hero-title-bg); color: #fff; font-size: clamp(32px, 4vw, 52px); line-height: 1.12; font-weight: 400; }
    .hero-subtitle { width: min(760px, 100%); margin-top: 14px; padding: 18px 22px; background: var(--surface-raised); border-radius: 5px; box-shadow: var(--shadow); color: var(--muted); font-size: 20px; line-height: 1.45; }
    .context-note, .appeal-box { padding: 18px; border: 1px solid var(--line); border-top: 5px solid var(--blue); background: var(--surface-raised); box-shadow: var(--shadow); color: var(--text); }
    .context-note strong, .appeal-box h2 { display: block; margin: 0 0 8px; color: var(--heading); font-size: 20px; line-height: 1.2; }
    .context-note p, .appeal-box p { margin: 0; color: var(--muted); font-size: 14px; line-height: 1.45; }
    .context-mobile, .appeal-short { display: none; }
    .context-note nav { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 14px; font-size: 13px; font-weight: 700; }
    .appeal-box { margin-bottom: 14px; }
    .appeal-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 14px; }
    .appeal-actions a { display: inline-flex; align-items: center; min-height: 34px; padding: 0 11px; border-radius: 4px; background: var(--control-bg); color: var(--control-text); font-size: 13px; font-weight: 700; text-decoration: none; }
    .appeal-actions a:hover, .appeal-actions a:focus-visible { background: var(--red); color: #fff; outline: none; }
    main { width: min(1320px, calc(100vw - 32px)); margin: 0 auto 70px; }
    .controls { position: sticky; top: 0; z-index: 2000; display: grid; grid-template-columns: minmax(200px, 1fr) 120px 170px 130px 130px 130px 95px auto; gap: 12px; align-items: end; padding: 16px; margin: 0 0 24px; background: var(--surface-raised); border: 1px solid var(--line); box-shadow: 0 2px 8px rgba(0,0,0,.12); }
    .chips { display: flex; flex-wrap: wrap; gap: 8px; margin: 0 0 14px; }
    .chip { display: inline-flex; align-items: center; gap: 5px; padding: 7px 16px; border: 1px solid var(--line); border-radius: 20px; background: var(--surface); color: var(--text); font-size: 13px; cursor: pointer; transition: .12s; }
    .chip:hover { border-color: var(--red); background: var(--row-hover); }
    .chip.active { background: var(--red); border-color: var(--red); color: #fff; }
    .sub-chips { display: flex; flex-wrap: wrap; gap: 6px; margin: -8px 0 14px; min-height: 30px; }
    .sub-chip { padding: 4px 12px; border: 1px solid var(--line); border-radius: 14px; background: var(--surface-muted); color: var(--muted); font-size: 12px; cursor: pointer; transition: .12s; }
    .sub-chip:hover { border-color: var(--red); color: var(--text); background: var(--surface); }
    .sub-chip.active { background: var(--red); border-color: var(--red); color: #fff; }
    .filter-toggle { display: none; }
    label { display: block; margin-bottom: 6px; color: var(--muted); font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; }
    input, select, button { font: inherit; }
    input, select { width: 100%; height: 44px; padding: 0 12px; border: 1px solid var(--line); border-radius: 5px; background: var(--input-bg); color: var(--input-text); outline: none; }
    input:focus, select:focus { border-color: var(--blue); box-shadow: 0 0 0 3px var(--focus-blue); }
    .reset { height: 34px; border: 0; border-radius: 5px; padding: 0 10px; background: var(--control-bg); color: var(--control-text); font-size: 13px; font-weight: 700; cursor: pointer; align-self: center; }
    .grid { display: grid; gap: 16px; }
    .kpis { grid-template-columns: repeat(5, minmax(0, 1fr)); }
    .card { background: var(--surface); border: 1px solid var(--line); overflow: hidden; }
    .seo-summary { padding: 18px; margin-top: 18px; line-height: 1.55; }
    .seo-summary h2, .seo-top h3 { margin: 0 0 8px; color: var(--heading); }
    .seo-summary p { margin: 0 0 12px; color: var(--muted); }
    .seo-facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; margin: 14px 0; padding: 0; list-style: none; }
    .seo-facts li { margin: 0; padding: 12px; border: 1px solid var(--line); background: var(--surface-muted); }
    .seo-facts strong { display: block; color: var(--heading); font-size: 22px; line-height: 1.1; }
    .seo-top { margin-top: 14px; }
    .seo-top ol { margin: 8px 0 0 22px; padding: 0; }
    .seo-top li { margin: 6px 0; }
    .seo-meta { color: var(--muted); }
    .kpi { padding: 18px; border-top: 5px solid var(--red); }
    .kpi:nth-child(2) { border-top-color: var(--orange); }
    .kpi:nth-child(3) { border-top-color: var(--blue); }
    .kpi:nth-child(4) { border-top-color: var(--red-dark); }
    .kpi:nth-child(5) { border-top-color: var(--green); }
    .kpi .value { display: block; color: var(--heading); font-size: clamp(30px, 3vw, 44px); font-weight: 400; line-height: 1; }
    .kpi .label { display: block; margin-top: 8px; color: var(--muted); font-size: 14px; }
    .panel-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); align-items: stretch; margin-top: 18px; }
    .panel { padding: 18px; min-height: 330px; }
    .panel h2, .dist h2 { margin: 0 0 8px; color: var(--heading); font-size: 22px; font-weight: 700; }
    .panel p { margin: 0 0 16px; color: var(--muted); font-size: 13px; }
    .bars { display: grid; gap: 10px; }
    .bar-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 7px 10px; align-items: center; font-size: 12px; }
    .bar-link { width: 100%; padding: 0; border: 0; background: transparent; color: inherit; text-align: left; text-decoration: none; cursor: pointer; }
    .bar-link:hover .bar-name, .bar-link:focus .bar-name { color: var(--red); text-decoration: underline; }
    .bar-link:focus { outline: 2px solid var(--focus-red); outline-offset: 3px; }
    .bar-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
    .bar-value { color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
    .track { grid-column: 1 / -1; height: 8px; background: var(--track-bg); overflow: hidden; }
    .fill { height: 100%; background: var(--red); }
    .fill.orange { background: var(--orange); }
    .fill.green { background: var(--green); }
    .dist { padding: 18px; margin-top: 18px; }
    .dist-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 22px; }
    .dist-panel p { margin: 0 0 12px; color: var(--muted); font-size: 13px; line-height: 1.45; }
    .dist-row { display: grid; grid-template-columns: 130px minmax(0, 1fr) 90px; gap: 12px; align-items: center; margin: 8px 0; font-size: 13px; }
    .bezirk-summary { padding: 18px; margin-top: 18px; }
    .bezirk-summary h2 { margin: 0 0 8px; color: var(--heading); font-size: 22px; font-weight: 700; }
    .bezirk-summary p { margin: 0 0 14px; color: var(--muted); font-size: 13px; }
    .bezirk-list { display: grid; gap: 8px; }
    .bezirk-row { display: grid; grid-template-columns: minmax(0, 1fr) 76px 80px 86px; gap: 12px; align-items: center; width: 100%; padding: 10px 12px; border: 1px solid var(--line); background: var(--surface); color: var(--heading); text-align: left; cursor: pointer; }
    .bezirk-row:hover, .bezirk-row:focus { border-color: var(--red); background: var(--row-hover); outline: none; }
    .bezirk-row strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .bezirk-row span { color: var(--muted); font-variant-numeric: tabular-nums; text-align: right; }
    .dist-track { height: 12px; background: var(--track-bg); overflow: hidden; }
    .dist-fill { height: 100%; background: var(--red); }
    .dist-note { margin-top: 10px !important; color: var(--muted); font-size: 12px !important; }
    .map-panel { margin-top: 18px; padding: 18px; }
    .map-panel h2 { margin: 0 0 8px; color: var(--heading); font-size: 22px; font-weight: 700; }
    .map-panel p { margin: 0 0 16px; color: var(--muted); font-size: 13px; }
    #placesMap { position: relative; z-index: 0; height: 520px; border: 1px solid var(--line); background: var(--map-bg); }
    #placesMap.map-needs-key::after, #placesMap.map-active::after { position: absolute; left: 50%; top: 18px; z-index: 1000; transform: translateX(-50%); padding: 10px 14px; border-radius: 4px; background: var(--hint-bg); color: #fff; font-size: 13px; font-weight: 700; box-shadow: 0 2px 10px rgba(0,0,0,.22); pointer-events: none; }
    #placesMap.map-needs-key::after { content: "Strg/⌘ halten, um mit dem Mausrad zu zoomen"; }
    #placesMap.map-active::after { content: "Karten-Zoom aktiv"; }
    @media (pointer: coarse) { #placesMap.map-needs-key::after { content: "Zwei Finger zum Zoomen und Bewegen der Karte"; } }
    .map-empty { display: grid; place-items: center; height: 100%; padding: 20px; color: var(--muted); text-align: center; }
    .map-consent { display: grid; place-items: center; height: 100%; padding: 24px; color: var(--muted); text-align: center; }
    .map-consent-inner { max-width: 520px; }
    .map-consent .map-consent-text { margin: 0 0 18px; color: var(--text); font-size: 14px; line-height: 1.5; }
    .map-consent-btn:not([hidden]) { display: inline-flex; align-items: center; min-height: 38px; padding: 0 16px; border: 0; border-radius: 4px; background: var(--red); color: #fff; font-size: 14px; font-weight: 700; cursor: pointer; }
    .map-consent-btn:hover, .map-consent-btn:focus-visible { background: var(--red-dark); outline: none; }
    .map-consent .map-consent-link { margin: 22px 0 0; font-size: 12px; line-height: 1.5; }
    .map-legend { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 10px; color: var(--muted); font-size: 13px; }
    .legend-dot { display: inline-block; width: 12px; height: 12px; margin-right: 6px; border-radius: 50%; vertical-align: -1px; }
    .legend-area { display: inline-block; width: 16px; height: 12px; margin-right: 6px; border: 2px solid var(--red); background: rgba(207,42,27,.12); vertical-align: -2px; }
    .leaflet-container { background: var(--map-bg); color: var(--text); }
    .leaflet-bar a, .leaflet-bar a:hover, .leaflet-control-attribution { background: var(--surface-raised); color: var(--text); border-color: var(--line); }
    .leaflet-control-attribution a, .leaflet-tooltip a { color: var(--red); }
    .leaflet-tooltip, .leaflet-popup-content-wrapper, .leaflet-popup-tip { background: var(--surface-raised); color: var(--text); border-color: var(--line); box-shadow: var(--shadow); }
    .tabs { display: flex; flex-wrap: wrap; gap: 8px; margin: 22px 0 14px; }
    .tab { border: 1px solid var(--line); border-radius: 5px; padding: 10px 14px; background: var(--surface-muted); color: var(--heading); font-weight: 700; cursor: pointer; }
    .tab.active { background: var(--red); border-color: var(--red); color: #fff; }
    .tab:disabled { opacity: .65; cursor: progress; }
    .nearby-status { margin: -6px 0 14px; color: var(--muted); font-size: 13px; }
    .nearby-status.error { color: var(--red); font-weight: 700; }
    .nearby-status[hidden] { display: none; }
    .table-head { display: flex; justify-content: space-between; align-items: center; margin: 0 0 10px; color: var(--muted); font-size: 14px; }
    .table-head strong { color: var(--heading); font-size: 22px; }
    .table-wrap { overflow: auto; background: var(--surface); border: 1px solid var(--line); }
    table { width: 100%; min-width: 1570px; border-collapse: collapse; table-layout: fixed; }
    col.rank { width: 70px; } col.name { width: 360px; } col.bezirk { width: 210px; } col.plz { width: 90px; } col.rating { width: 95px; } col.reviews { width: 125px; } col.banner { width: 100px; } col.removed { width: 120px; } col.estimate { width: 125px; } col.ratio { width: 120px; } col.scenario { width: 170px; } col.checked { width: 130px; } col.category { width: 175px; }
    th { position: sticky; top: 0; z-index: 2; padding: 0; background: var(--table-head-bg); border-bottom: 2px solid var(--line); color: var(--heading); font-size: 13px; text-align: left; }
    th button { display: flex; align-items: center; gap: 5px; width: 100%; min-height: 44px; padding: 12px; border: 0; background: transparent; color: inherit; font: inherit; font-weight: 700; text-align: inherit; cursor: pointer; }
    th.num button, th.rank button { justify-content: flex-end; text-align: right; }
    th button:hover { color: var(--red); background: var(--table-head-hover); }
    .arrow { width: 1em; color: var(--muted); }
    button.active .arrow { color: var(--red); }
    td { padding: 12px; border-bottom: 1px solid var(--line); vertical-align: top; font-size: 14px; }
    tbody tr:nth-child(even) { background: var(--surface-subtle); }
    tbody tr:hover { background: var(--row-hover); }
    tbody tr.target-row { background: var(--target-row); box-shadow: inset 5px 0 0 var(--red); }
    td.num, td.rank { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    td.name { overflow-wrap: anywhere; font-weight: 700; }
    .entry-address { display: block; margin-top: 4px; color: var(--muted); font-size: 12px; font-weight: 400; line-height: 1.35; }
    a { color: var(--red); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .pill { display: inline-flex; align-items: center; border-radius: 3px; padding: 3px 7px; background: var(--pill-bg); color: var(--pill-text); font-weight: 700; font-size: 12px; }
    .pill.bad { background: var(--pill-bad-bg); color: var(--pill-bad-text); }
    .show-more { display: block; min-width: 220px; height: 42px; margin: 14px auto 0; padding: 0 18px; border: 1px solid var(--line); border-radius: 999px; background: var(--surface-raised); color: var(--heading); font-weight: 700; cursor: pointer; }
    .show-more:hover, .show-more:focus-visible { border-color: var(--red); color: var(--red); outline: none; }
    .show-more[hidden] { display: none; }
    .related-dashboards { padding: 18px; margin-top: 18px; }
    .related-dashboards h2 { margin: 0 0 8px; color: var(--heading); font-size: 22px; font-weight: 700; }
    .related-dashboards p { margin: 0 0 14px; color: var(--muted); font-size: 13px; line-height: 1.45; }
    .related-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
    .related-link { display: block; min-height: 82px; padding: 12px; border: 1px solid var(--line); background: var(--surface-muted); color: var(--text); text-decoration: none; }
    .related-link:hover, .related-link:focus-visible { border-color: var(--red); background: var(--row-hover); outline: none; text-decoration: none; }
    .related-link strong { display: block; margin-bottom: 6px; color: var(--heading); font-size: 15px; line-height: 1.2; }
    .related-link span { display: block; color: var(--muted); font-size: 12px; line-height: 1.35; overflow-wrap: anywhere; }
    footer { margin-top: 18px; color: var(--muted); font-size: 13px; line-height: 1.5; }
    .footer-privacy, .footer-credit { margin-top: 6px; }
    .footer-credit a { font-weight: 700; }
    @media (min-width: 1600px) {
      .table-wrap { margin: 0 calc(-1 * (100vw - min(1320px, calc(100vw - 32px))) / 2); }
      table { min-width: 0; table-layout: auto; }
      col.rank, col.name, col.bezirk, col.plz, col.rating, col.reviews, col.banner, col.removed, col.estimate, col.ratio, col.scenario, col.checked, col.category { width: auto; }
    }
    @media (max-width: 1200px) { .hero-inner { grid-template-columns: 1fr; } .appeal-box { width: min(760px, 100%); } .kpis, .panel-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .related-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } .controls { grid-template-columns: 1fr 1fr 1fr; } .search { grid-column: 1 / -1; } .theme-toggle { margin-left: auto; margin-right: 0; } .n-logo { position: relative; height: 76px; width: 150px; margin-left: 0; padding-top: 48px; } .n-logo::before { top: 40px; } .n-logo::after { top: 4px; } }
    @media (max-width: 720px) {
      .sitebar-inner, main, .hero-inner { width: min(100vw - 20px, 1320px); }
      .sitebar-inner { gap: 14px; }
      .top-icons { display: none; }
      .theme-toggle { width: 42px; padding: 0; justify-content: center; }
      .theme-toggle-text { display: none; }
      .n-logo { width: 128px; font-size: 18px; padding-left: 10px; padding-right: 10px; }
      .kpis, .panel-grid, .seo-facts, .dist-grid, .related-grid { grid-template-columns: 1fr; }
      .controls { position: sticky; top: 0; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 9px 10px; padding: 10px; margin-bottom: 14px; }
      .filter-toggle { grid-column: 1 / -1; display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; min-height: 42px; padding: 8px 12px; border: 0; border-radius: 5px; background: var(--control-bg); color: var(--control-text); text-align: left; cursor: pointer; }
      .filter-toggle strong { display: block; font-size: 15px; line-height: 1.1; }
      .filter-summary { display: block; max-width: 380px; overflow: hidden; color: var(--control-muted); font-size: 12px; font-weight: 400; line-height: 1.3; text-overflow: ellipsis; white-space: nowrap; }
      .filter-toggle-icon { font-size: 16px; transition: transform .18s ease; }
      .controls:not(.is-collapsed) .filter-toggle-icon { transform: rotate(180deg); }
      .controls.is-collapsed .control, .controls.is-collapsed .reset { display: none; }
      .search { grid-column: 1 / -1; }
      label { margin-bottom: 4px; font-size: 10px; letter-spacing: .04em; }
      input, select { height: 38px; padding: 0 9px; font-size: 15px; }
      .reset { height: 34px; padding: 0 10px; font-size: 13px; }
      .hero { min-height: 300px; margin-bottom: 14px; }
      .hero-inner { padding-top: 92px; gap: 14px; }
      .hero-title { font-size: 32px; padding: 18px; }
      .hero-subtitle { display: none; }
      .context-note, .appeal-box { padding: 16px; }
      .context-note strong, .appeal-box h2 { font-size: 19px; }
      .context-desktop, .appeal-full { display: none; }
      .context-mobile, .appeal-short { display: inline; }
    }
  </style>
</head>
<body>
  <div class="sitebar" role="banner">
    <div class="sitebar-inner">
      <div class="top-icons" aria-hidden="true"><span>●</span><span>☝</span><span>▰</span></div>
      <button class="theme-toggle" id="themeToggle" type="button" aria-label="Dunkles Design aktivieren" aria-pressed="false"><span class="theme-toggle-icon" aria-hidden="true">☾</span><span class="theme-toggle-text">Dunkel</span></button>
      <div class="n-logo">__CITY__</div>
    </div>
  </div>

  <section class="hero" aria-label="Seitentitel">
    <div class="hero-inner">
      <div class="hero-copy">
        <h1 class="hero-title">__CITY__ Google-Maps-Bewertungen</h1>
        <div class="hero-subtitle">Interaktives Daten-Dashboard zu öffentlich sichtbaren Google-Maps-Hinweisen auf entfernte Bewertungen wegen Diffamierungsbeschwerden.</div>
      </div>
      <section class="context-note" aria-label="Einordnung">
        <strong>Einordnung</strong>
        <p><strong>Historische Momentaufnahme</strong> Letzter enthaltener Datenabruf: __SNAPSHOT__. Keine Live-Auskunft; der Zeitpunkt pro Eintrag ist maßgeblich.</p>
        <p class="context-desktop">Diese Seite dokumentiert öffentlich sichtbare Google-Hinweise. Sie behauptet nicht, dass Betriebe rechtswidrig gehandelt, Bewertungen missbräuchlich entfernen ließen oder legitime Kritik unterdrückt wurde. Die Daten sind eine Momentaufnahme.</p>
        <p class="context-mobile">Dokumentation öffentlich sichtbarer Google-Hinweise. Keine Aussage über Rechtswidrigkeit, Missbrauch oder unterdrückte Kritik. Momentaufnahme.</p>
        __LEGAL_NAV__
      </section>
    </div>
  </section>

  <main>
    <aside class="appeal-box" aria-label="Hilfe bei entfernter eigener Bewertung">
      <h2>Eigene Bewertung entfernt?</h2>
      <p><span class="appeal-full">Wenn eine eigene Google-Maps-Bewertung aus deiner Sicht zu Unrecht entfernt wurde, kannst du zuerst bei Google Einspruch einlegen. In der EU gibt es zusätzlich außergerichtliche Streitbeilegung nach dem Digital Services Act. Keine Erfolgsgarantie.</span><span class="appeal-short">Wenn deine eigene Bewertung aus deiner Sicht zu Unrecht entfernt wurde: erst Google-Einspruch, danach ggf. EU-Streitbeilegung. Keine Erfolgsgarantie.</span></p>
      <div class="appeal-actions"><a href="https://support.google.com/maps/answer/16673099?hl=de" target="_blank" rel="noopener noreferrer">Google-Einspruch</a><a href="https://digital-strategy.ec.europa.eu/de/policies/dsa-out-court-dispute-settlement" target="_blank" rel="noopener noreferrer">EU-Streitbeilegung</a></div>
    </aside>

    <section class="chips" aria-label="Quick-Filter"><button type="button" class="chip" data-chip="banner">🔴 Mit Google-Hinweis</button><button type="button" class="chip" data-chip="gastro">🍽️ Gastronomie</button><button type="button" class="chip" data-chip="nachtleben">🎉 Nachtleben</button><button type="button" class="chip" data-chip="beauty">💇 Beauty &amp; Wellness</button><button type="button" class="chip" data-chip="hotels">🏨 Beherbergung</button><button type="button" class="chip" data-chip="gesundheit">🏥 Gesundheit</button><button type="button" class="chip" data-chip="altstadt">🗺️ Altstadt</button></section>
    <section class="controls is-collapsed" id="dashboardFilterControls" aria-label="Dashboard-Filter">
      <button class="filter-toggle" id="filterToggle" type="button" aria-expanded="false" aria-controls="dashboardFilterControls"><span><strong>Filter</strong><span class="filter-summary" id="filterSummary">Keine aktiven Filter</span></span><span class="filter-toggle-icon" aria-hidden="true">▾</span></button>
      <div class="control search"><label for="searchInput">Suche</label><input id="searchInput" type="search" placeholder="Name, PLZ, Kategorie, Hinweisbereich …" autocomplete="off"></div>
      <div class="control"><label for="postcodeFilter">PLZ</label><select id="postcodeFilter"><option value="">Alle PLZ</option>__POSTCODE_OPTIONS__</select></div>
      <div class="control"><label for="bezirkFilter">Bezirk</label><select id="bezirkFilter"><option value="">Alle Bezirke</option>__BEZIRK_OPTIONS__</select></div>
      <div class="control"><label for="bannerFilter">Hinweis</label><select id="bannerFilter"><option value="all">Alle</option><option value="banner">Mit Hinweis</option><option value="clean">Ohne Hinweis</option></select></div>
      <div class="control"><label for="rangeFilter">Spanne</label><select id="rangeFilter"><option value="">Alle Bereiche</option>__RANGE_OPTIONS__</select></div>
      <div class="control"><label for="categoryFilter">Kategorie</label><select id="categoryFilter"><option value="">Alle Kategorien</option>__CATEGORY_OPTIONS__</select></div>
      <div class="control"><label for="minReviews">Min. Rezensionen</label><input id="minReviews" type="number" min="0" step="1" value="0"></div>
      <button class="reset" id="resetFilters" type="button">Reset</button>
    </section>

    <section class="grid kpis" aria-label="Kennzahlen">
      <div class="card kpi"><span class="value" id="kpiPlaces">–</span><span class="label">Orte im Filter</span></div>
      <div class="card kpi"><span class="value" id="kpiBanners">–</span><span class="label">mit beobachtetem Google-Hinweis</span></div>
      <div class="card kpi"><span class="value" id="kpiBannerPct">–</span><span class="label">Anteil mit Hinweis (Orte)</span></div>
      <div class="card kpi"><span class="value" id="kpiRemoved">–</span><span class="label">Näherungswert aus Spannen</span></div>
      <div class="card kpi"><span class="value" id="kpiClean">–</span><span class="label">ohne Hinweis beim Abruf</span></div>
    </section>

__SEO_SUMMARY__

    <section class="grid panel-grid" aria-label="Top-Rankings">
      <article class="card panel"><h2>Höchste Hinweisspannen</h2><p>Beobachtete Google-Spannen; innerhalb derselben Spanne alphabetisch. Keine Aussage über Betriebsqualität.</p><div class="bars" id="barsRemoved"></div></article>
      <article class="card panel"><h2>Geschätzter Modellanteil</h2><p>Modellanteil: Spannen-Näherungswert / (sichtbare Rezensionen + Näherungswert). Keine gemessene Löschquote; unterschiedliche Zeitbezüge.</p><div class="bars" id="barsRatio"></div></article>
      <article class="card panel"><h2>1-Stern-Szenario</h2><p>Modell: Spannenmittelpunkt; für „über 250“ der Rechenwert 300. Jeweils 1 Stern angenommen, nicht beobachtet. Kein tatsächliches Rating und keine gesicherte Untergrenze. Niedrigste Modellwerte zuerst; keine Aussage über Betriebsqualität.</p><div class="bars" id="barsScenario"></div></article>
      <article class="card panel"><h2>Ohne beobachteten Google-Hinweis</h2><p>Im Abruf war kein passender Google-Hinweis sichtbar. Sortiert nach Rating ab 100 Rezensionen. Keine Qualitätsaussage gegenüber anderen Orten.</p><div class="bars" id="barsClean"></div></article>
    </section>

    <section class="card dist" aria-label="Verteilungen">
      <div class="dist-grid">
        <div class="dist-panel"><h2>Verteilung der Spannen</h2><div id="distribution"></div></div>
        <div class="dist-panel"><h2>Modellanteile im Vergleich</h2><p>Keine gemessene Löschquote. Nur Orte mit beobachtetem Google-Hinweis und mindestens 50 sichtbaren Rezensionen im aktuellen Filter.</p><div id="ratioHistogram"></div></div>
      </div>
    </section>

    <section class="card bezirk-summary" aria-label="Bezirks-Gruppen"><h2>Gruppierung nach statistischem Bezirk</h2><p>Bezirke im aktuellen Filter, sortiert nach Anteil der Orte mit Hinweis (nicht nach Modellanteil). Anklicken setzt den Bezirksfilter.</p><div class="bezirk-list" id="bezirkSummary"></div></section>

    <section class="card bezirk-summary" aria-label="Kategorie-Gruppen"><h2>Gruppierung nach Kategorie</h2><p>Kategorie-Gruppen im aktuellen Filter, sortiert nach Anteil der Orte mit Hinweis (nicht nach Modellanteil).</p><div class="bezirk-list" id="parentSummary"></div></section>

    <section class="card map-panel" aria-label="Karte">
      <h2>Karte der erfassten Orte</h2>
      <p><span id="mapCount">–</span> Orte mit Koordinaten im aktuellen Filter. Marker anklicken markiert Einträge; Bezirksflächen anklicken setzt den Bezirkfilter.</p>
      <div id="placesMap">__MAP_CONSENT__</div>
      <button id="mapConsentRevoke" class="map-consent-btn" type="button" hidden>Karte deaktivieren</button>
      <div class="map-legend"><span><i class="legend-area"></i>Bezirk, klickbar</span><span><i class="legend-dot" style="background:#1f6f8b"></i>dein Standort</span><span><i class="legend-dot" style="background:#3c4e6b"></i>hoher Modellanteil</span><span><i class="legend-dot" style="background:#5b7b8e"></i>sichtbarer Hinweis</span><span><i class="legend-dot" style="background:#8a8e94"></i>kein sichtbarer Hinweis</span></div>
    </section>

    <nav class="tabs" aria-label="Tabellen-Presets">
      <button class="tab active" data-mode="removed">Höchste Spannen</button>
      <button class="tab" data-mode="ratio">Modellanteil</button>
      <button class="tab" data-mode="scenario">1-Stern-Szenario</button>
      <button class="tab" data-mode="clean">Ohne Hinweis</button>
      <button class="tab" data-mode="nearby">In meiner Nähe</button>
    </nav>
    <div class="sub-chips" id="subChips"></div>
    <div class="nearby-status" id="nearbyStatus" role="status" aria-live="polite" hidden></div>

    <div class="table-head"><strong id="tableTitle">Höchste Hinweisspannen</strong><span id="resultCount">–</span></div>
    <p id="scenarioExplanation">1-Stern-Szenario: angenommene Anzahl entfernter Rezensionen (Spannenmittelpunkt; für „über 250“ der Rechenwert 300), jeweils mit 1 Stern. Kein tatsächliches Rating und keine gesicherte Untergrenze. Anzahl und Sterne der entfernten Rezensionen sind unbekannt.</p>
    <section class="table-wrap" aria-label="Daten-Explorer">
      <table id="placesTable">
        <colgroup><col class="rank"><col class="name"><col class="bezirk"><col class="plz"><col class="rating"><col class="reviews"><col class="banner"><col class="removed"><col class="estimate"><col class="ratio"><col class="scenario"><col class="checked"><col class="category"></colgroup>
        <thead><tr>
          <th class="rank"><button data-sort="rank">Rang <span class="arrow"></span></button></th>
          <th><button data-sort="name">Name / Google Maps <span class="arrow"></span></button></th>
          <th><button data-sort="bezirkLabel">Bezirk <span class="arrow"></span></button></th>
          <th><button data-sort="postcode">PLZ <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="rating">Rating <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="reviewCount">Rezensionen <span class="arrow"></span></button></th>
          <th><button data-sort="hasBanner">Google-Hinweis <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="removedEstimate">Spanne <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="removedEstimate">Näherungswert <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="deletionRatioPct">Modellanteil <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="oneStarScenario" aria-describedby="scenarioExplanation" title="Modell mit angenommener Anzahl entfernter Rezensionen und jeweils 1 Stern; kein tatsächliches Rating und keine gesicherte Untergrenze.">1-Stern-Szenario (Modell) <span class="arrow"></span></button></th>
          <th><button data-sort="readAt">Datenabruf <span class="arrow"></span></button></th>
          <th><button data-sort="category">Kategorie <span class="arrow"></span></button></th>
        </tr></thead>
        <tbody></tbody>
      </table>
    </section>
    <button class="show-more" id="showMoreRows" type="button" hidden>Mehr Zeilen anzeigen</button>

    <section class="card related-dashboards" aria-label="Weitere regionale Dashboards">
      <h2>Weitere regionale Dashboards</h2>
      <p>Öffentlich verfügbare Dashboards anderer Regionen zum gleichen Google-Maps-Hinweis-Thema.</p>
      <div class="related-grid">
        <a class="related-link" href="https://bewertungsradar-saar.de/" target="_blank" rel="noopener noreferrer"><strong>Saarland</strong><span>bewertungsradar-saar.de</span></a>
        <a class="related-link" href="https://nekronomekron.github.io/landshut-maps-review-removals/" target="_blank" rel="noopener noreferrer"><strong>Landshut</strong><span>nekronomekron.github.io</span></a>
        <a class="related-link" href="https://cooljackgi.github.io/giessen-maps-review-removals/" target="_blank" rel="noopener noreferrer"><strong>Gießen</strong><span>cooljackgi.github.io</span></a>
        <a class="related-link" href="https://berlintrustindex.com/" target="_blank" rel="noopener noreferrer"><strong>Berlin Trust Index</strong><span>berlintrustindex.com</span></a>
        <a class="related-link" href="https://hermes7654.github.io/ibbenbueren-review-removals/" target="_blank" rel="noopener noreferrer"><strong>Ibbenbüren / Münster</strong><span>hermes7654.github.io</span></a>
      </div>
    </section>

    <footer>
      <div>Historische Recherche-Momentaufnahme; keine Rezensionstexte und keine separate Rohdaten-CSV. Die im Dashboard enthaltenen Profildaten bleiben technisch auslesbar. Dashboard basiert auf der MIT-lizenzierten Vorlage von Patrick Wozniak.</div>
      <div>Quelle: Google Maps, öffentlich sichtbare Hinweise. „Kein Hinweis“ heißt nur: im Abruf war kein passender Hinweis sichtbar. Letzter enthaltener Datenabruf: __SNAPSHOT__.</div>
__ANALYTICS_PRIVACY__
      __FOOTER_LEGAL_LINKS__
      <div class="footer-credit">© 2026 Patrick Wozniak · <a href="https://patwoz.dev" target="_blank" rel="noopener noreferrer">patwoz.dev</a></div>
    </footer>
  </main>

  <script id="dashboardConfig" type="application/json">__CONFIG__</script>
  <script id="placesData" type="application/json">__DATA__</script>
  <script id="bezirkData" type="application/json">__BEZIRK_DATA__</script>
  <script>
/* __LEAFLET_LICENSE__ */
__LEAFLET_JS__
  </script>
  <script>
__DASHBOARD_JS__
  </script>
</body>
</html>`

	return strings.NewReplacer(
		"__PAGE_TITLE__", esc(meta.PageTitle),
		"__PAGE_DESCRIPTION__", escAttr(meta.PageDescription),
		"__CANONICAL_URL__", escAttr(meta.CanonicalURL),
		"__SITE_NAME__", esc(meta.SiteName),
		"__STRUCTURED_DATA__", structuredData,
		"__CITY__", esc(city),
		"__LEGAL_NAV__", legalNavHTML(args),
		"__FOOTER_LEGAL_LINKS__", footerLegalLinksHTML(args),
		"__MAP_CONSENT__", mapConsentHTML(args),
		"__SEO_SUMMARY__", seoSummaryHTML(stats, city),
		"__POSTCODE_OPTIONS__", postcodeOptions,
		"__BEZIRK_OPTIONS__", bezirkOptions,
		"__RANGE_OPTIONS__", rangeOptions,
		"__CATEGORY_OPTIONS__", categoryOptions,
		"__DASHBOARD_JS__", dashboardJS,
		"__LEAFLET_CSS__", leafletCSS,
		"__LEAFLET_JS__", strings.ReplaceAll(leafletJS, "//# sourceMappingURL=leaflet.js.map", ""),
		"__LEAFLET_LICENSE__", leafletLicense,
		"__ANALYTICS__", plausibleAnalyticsSnippet(args),
		"__ANALYTICS_PRIVACY__", plausiblePrivacyNotice(args),
		"__SNAPSHOT__", snapshotDisplay,
		"__CONFIG__", configText,
		"__DATA__", jsonText,
		"__BEZIRK_DATA__", bezirkText,
	).Replace(page)
}

type compactClientData struct {
	Dictionaries [][]string      `json:"d"`
	Rows         [][]interface{} `json:"r"`
}

type stringDictionary struct {
	values []string
	index  map[string]int
}

const compactCoordScale = 100000.0

func compactClientDataJSON(data []clientRow, city string) string {
	postcodes := newStringDictionary()
	bezirke := newStringDictionary()
	categories := newStringDictionary()
	parentCategories := newStringDictionary()
	ranges := newStringDictionary()

	rows := make([][]interface{}, 0, len(data))
	for _, row := range data {
		compactRow := []interface{}{
			compactCID(row.ID),
			row.Name,
			postcodes.Add(row.Postcode),
			compactCoordinatePtr(row.Lat, 49),
			compactCoordinatePtr(row.Lng, 11),
			bezirke.Add(row.BezirkLabel),
			compactFloatPtr(row.Rating, 1),
			compactIntPtr(row.ReviewCount),
			categories.Add(row.Category),
			parentCategories.Add(row.ParentCategory),
			compactAddress(row.Address, row.Postcode, city),
			readAtMinute(row.ReadAt),
		}
		if row.HasBanner {
			compactRow = append(compactRow,
				ranges.Add(row.RemovedRange),
				compactFloat(row.RemovedEstimate, 1),
				compactFloatPtr(row.DeletionRatioPct, 2),
			)
		}
		rows = append(rows, compactRow)
	}
	return safeJSON(compactClientData{
		Dictionaries: [][]string{postcodes.values, bezirke.values, categories.values, parentCategories.values, ranges.values},
		Rows:         rows,
	})
}

func compactBezirkDataJSON(boundaries []mapsreview.BezirkBoundary) string {
	rows := make([][]interface{}, 0, len(boundaries))
	for _, boundary := range boundaries {
		polygons := make([][]int, 0, len(boundary.Polygons))
		for _, polygon := range boundary.Polygons {
			flat := make([]int, 0, len(polygon)*2)
			prevLat := 0
			prevLng := 0
			for _, point := range polygon {
				if len(point) < 2 {
					continue
				}
				lat := compactCoordinate(point[0], 49)
				lng := compactCoordinate(point[1], 11)
				if len(flat) == 0 {
					flat = append(flat, lat, lng)
				} else {
					flat = append(flat, lat-prevLat, lng-prevLng)
				}
				prevLat = lat
				prevLng = lng
			}
			if len(flat) > 0 {
				polygons = append(polygons, flat)
			}
		}
		rows = append(rows, []interface{}{boundary.Label, polygons})
	}
	return safeJSON(rows)
}

func newStringDictionary() *stringDictionary {
	return &stringDictionary{index: map[string]int{}}
}

func (d *stringDictionary) Add(value string) int {
	if index, ok := d.index[value]; ok {
		return index
	}
	index := len(d.values)
	d.index[value] = index
	d.values = append(d.values, value)
	return index
}

func compactCID(id string) string {
	if _, after, ok := strings.Cut(id, ":"); ok {
		return strings.TrimPrefix(after, "0x")
	}
	return strings.TrimPrefix(id, "0x")
}

func compactCoordinatePtr(value *float64, base float64) interface{} {
	if value == nil {
		return nil
	}
	return compactCoordinate(*value, base)
}

func compactCoordinate(value float64, base float64) int {
	return int(math.Round((value - base) * compactCoordScale))
}

func compactAddress(address string, postcode string, cityName string) string {
	if address == "" {
		return ""
	}
	city := strings.TrimSpace(postcode + " " + cityName)
	if address == city {
		return ""
	}
	if before, ok := strings.CutSuffix(address, ", "+city); ok {
		return before
	}
	return "!" + address
}

func compactFloatPtr(value *float64, decimals int) interface{} {
	if value == nil {
		return nil
	}
	return compactFloat(*value, decimals)
}

func compactFloat(value float64, decimals int) interface{} {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return roundDecimal(value, decimals)
}

func compactIntPtr(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func roundDecimal(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}

func readAtMinute(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, value)
	}
	if err != nil {
		return 0
	}
	return parsed.Unix() / 60
}

func safeJSON(value interface{}) string {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return strings.ReplaceAll(string(jsonData), "<", "\\u003c")
}

func snapshotTime(data []clientRow) time.Time {
	latest := time.Time{}
	for _, row := range data {
		if row.ReadAt == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, row.ReadAt)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, row.ReadAt)
		}
		if err == nil && parsed.After(latest) {
			latest = parsed
		}
	}
	return latest.UTC()
}

func makeSEOStats(data []clientRow, snapshot string) seoStats {
	stats := seoStats{Total: len(data), Snapshot: snapshot}
	estimate := 0.0
	var first, last time.Time
	for _, row := range data {
		if t, err := time.Parse(time.RFC3339Nano, row.ReadAt); err == nil {
			if first.IsZero() || t.Before(first) {
				first = t
			}
			if last.IsZero() || t.After(last) {
				last = t
			}
		}
		if row.HasBanner {
			stats.Banners++
			estimate += row.RemovedEstimate
			// Sum before rounding; rounding every row inflates the total.
		} else {
			stats.Clean++
		}
	}

	if !first.IsZero() {
		stats.Coverage = first.UTC().Format("2006-01-02") + "/" + last.UTC().Format("2006-01-02")
	}
	stats.RemovedEstimate = int(math.Round(estimate))
	top := make([]clientRow, 0, len(data))
	for _, row := range data {
		if row.HasBanner && row.RemovedEstimate > 0 {
			top = append(top, row)
		}
	}
	sort.SliceStable(top, func(i, j int) bool {
		if top[i].RemovedEstimate != top[j].RemovedEstimate {
			return top[i].RemovedEstimate > top[j].RemovedEstimate
		}
		return top[i].Name < top[j].Name
	})
	if len(top) > 8 {
		top = top[:8]
	}
	stats.Top = top
	return stats
}

func seoSummaryHTML(stats seoStats, city string) string {
	return fmt.Sprintf(`<section class="card seo-summary" aria-labelledby="data-overview-title">
      <h2 id="data-overview-title">Datenstand: Google-Maps-Bewertungen und Google-Hinweise in %s</h2>
      <p>Dieses Dashboard macht öffentlich sichtbare Hinweise auf wegen Diffamierungsbeschwerden entfernte Google-Maps-Bewertungen in %s durchsuchbar. Die Karte, Filter und Ranglisten zeigen sichtbare Google-Hinweise, öffentlich angezeigte Spannen, Näherungswerte, ausdrücklich als Modell gekennzeichnete Anteile und ein hypothetisches 1-Stern-Szenario je Ort.</p>
      <ul class="seo-facts">
        <li><strong>%s</strong> erfasste Orte</li>
        <li><strong>%s</strong> mit beobachtetem Google-Hinweis</li>
        <li><strong>%s</strong> Näherungswert aus Spannen</li>
        <li><strong>%s</strong> ohne Hinweis beim Abruf</li>
      </ul>
      <p class="seo-meta">Datenstand: %s. Kein sichtbarer Hinweis bedeutet nur, dass beim Abruf kein passender Hinweis sichtbar war.</p>
%s
    </section>`,
		esc(city),
		esc(city),
		mapsreview.FormatGermanInt(stats.Total),
		mapsreview.FormatGermanInt(stats.Banners),
		mapsreview.FormatGermanInt(stats.RemovedEstimate),
		mapsreview.FormatGermanInt(stats.Clean),
		esc(stats.Snapshot),
		seoTopListHTML(stats.Top),
	)
}

func seoTopListHTML(rows []clientRow) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="seo-top"><h3>Kurzüberblick: Orte mit den höchsten Hinweisspannen</h3><ol>`)
	for _, row := range rows {
		label := esc(row.Name)
		if row.URL != "" {
			label = fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer nofollow">%s</a>`, escAttr(row.URL), label)
		}
		meta := []string{}
		if row.RemovedRange != "" {
			meta = append(meta, "Spanne "+row.RemovedRange)
		}
		if row.Postcode != "" {
			meta = append(meta, "PLZ "+row.Postcode)
		}
		if row.BezirkLabel != "" {
			meta = append(meta, row.BezirkLabel)
		}
		detail := ""
		if len(meta) > 0 {
			detail = esc(strings.Join(meta, " · ")) + " · "
		}
		fmt.Fprintf(&b, `<li>%s <span class="seo-meta">%sNäherungswert ca. %s</span></li>`,
			label,
			detail,
			mapsreview.FormatGermanInt(int(row.RemovedEstimate+0.5)),
		)
	}
	b.WriteString(`</ol></div>`)
	return b.String()
}

func structuredDataJSON(stats seoStats, snapshot time.Time, city string, meta dashboardMetadata) string {
	data := map[string]interface{}{
		"@context": "https://schema.org",
		"@graph": []map[string]interface{}{
			{
				"@type":      "WebSite",
				"@id":        meta.CanonicalURL + "#website",
				"url":        meta.CanonicalURL,
				"name":       meta.SiteName,
				"inLanguage": "de-DE",
				"publisher": map[string]interface{}{
					"@type": "Person",
					"name":  "Patrick Wozniak",
					"url":   "https://patwoz.dev",
				},
			},
			{
				"@type":       "WebPage",
				"@id":         meta.CanonicalURL + "#webpage",
				"url":         meta.CanonicalURL,
				"name":        meta.PageTitle,
				"description": meta.PageDescription,
				"isPartOf": map[string]interface{}{
					"@id": meta.CanonicalURL + "#website",
				},
				"mainEntity": map[string]interface{}{
					"@id": meta.CanonicalURL + "#dataset",
				},
				"inLanguage": "de-DE",
			},
			{
				"@type":                "Dataset",
				"@id":                  meta.CanonicalURL + "#dataset",
				"name":                 fmt.Sprintf("%s Google-Maps-Bewertungen mit sichtbaren Google-Hinweisen", city),
				"description":          meta.PageDescription,
				"url":                  meta.CanonicalURL,
				"measurementTechnique": "Scrape öffentlich sichtbarer Google-Maps-Ortsseiten",
				"keywords": []string{
					city,
					"Google Maps Bewertungen",
					"entfernte Bewertungen",
					"Google-Hinweise",
					"Diffamierungsbeschwerden",
				},
				"spatialCoverage": map[string]interface{}{
					"@type": "City",
					"name":  city,
				},
				"variableMeasured": []string{
					"Orte",
					"sichtbare Google-Hinweise",
					"Näherungswert aus Google-Spannen",
					"Modellanteil (keine gemessene Quote)",
					"1-Stern-Szenario (Modellwert, kein tatsächliches Rating)",
				},
				"size": fmt.Sprintf("%d Orte, %d sichtbare Google-Hinweise", stats.Total, stats.Banners),
				"creator": map[string]interface{}{
					"@type": "Person",
					"name":  "Patrick Wozniak",
					"url":   "https://patwoz.dev",
				},
			},
		},
	}
	if !snapshot.IsZero() {
		graph := data["@graph"].([]map[string]interface{})
		graph[2]["temporalCoverage"] = stats.Coverage
	}
	jsonData, _ := json.MarshalIndent(data, "  ", "  ")
	return strings.ReplaceAll(string(jsonData), "<", "\\u003c")
}

func plausibleAnalyticsSnippet(a args) string {
	src := plausibleAnalyticsSrc(a)
	if src == "" {
		return ""
	}
	config := safeJSON(map[string]string{"src": src, "domain": plausibleAnalyticsDomain(a)})
	return `<script id="analyticsConfig" type="application/json">` + config + `</script><script>` + analyticsJS + `</script>`
}

func plausiblePrivacyNotice(a args) string {
	if plausibleAnalyticsSrc(a) == "" {
		return ""
	}
	return fmt.Sprintf(`<div class="footer-privacy">Optionale Reichweitenanalyse mit Plausible. Erst nach Einwilligung werden Nutzungsereignisse an <code>%s</code> übertragen. %s <a href="datenschutz.html">Datenschutzdetails</a>. Die Auswahl gilt für diesen Tab; Widerruf beendet weitere Erfassung, nicht bereits erfolgte Übermittlungen. <button id="analyticsConsent" type="button">Analytics erlauben</button><span id="analyticsConsentStatus" role="status"></span></div>`, esc(analyticsHost(a.AnalyticsSrc)), esc(a.AnalyticsPrivacyText))
}

func plausibleAnalyticsSrc(args args) string {
	if !args.AnalyticsEnabled || args.AnalyticsPrivacyText == "" || !legalPagesEnabled(args) {
		return ""
	}
	return strings.TrimSpace(args.AnalyticsSrc)
}

func plausibleAnalyticsDomain(args args) string {
	return strings.TrimSpace(args.AnalyticsDomain)
}

func analyticsHost(src string) string {
	if _, after, ok := strings.Cut(src, "://"); ok {
		src = after
	}
	host, _, _ := strings.Cut(src, "/")
	return host
}

func allBezirkLabels() []string {
	bezirke := mapsreview.AllBezirke()
	out := make([]string, 0, len(bezirke))
	for _, bezirk := range bezirke {
		out = append(out, bezirk.ID+" "+bezirk.Name)
	}
	sort.Strings(out)
	return out
}

func uniqueSorted(data []clientRow, value func(clientRow) string) []string {
	set := map[string]bool{}
	for _, row := range data {
		v := value(row)
		if v != "" {
			set[v] = true
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func countRows(data []clientRow, keep func(clientRow) bool) int {
	count := 0
	for _, row := range data {
		if keep(row) {
			count++
		}
	}
	return count
}

func maxEstimateForRange(data []clientRow, r string) float64 {
	max := 0.0
	for _, row := range data {
		if row.RemovedRange == r && row.RemovedEstimate > max {
			max = row.RemovedEstimate
		}
	}
	return max
}

func esc(value string) string     { return html.EscapeString(value) }
func escAttr(value string) string { return html.EscapeString(value) }

package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

const (
	defaultInput  = mapsreview.ResultsJSON
	defaultOutput = "output/charts/nuernberg_dashboard.html"
)

type args struct {
	Input  string
	Output string
}

type clientRow struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Postcode           string   `json:"postcode"`
	Lat                *float64 `json:"lat,omitempty"`
	Lng                *float64 `json:"lng,omitempty"`
	Rating             *float64 `json:"rating"`
	ReviewCount        *int     `json:"reviewCount"`
	Category           string   `json:"category"`
	HasBanner          bool     `json:"hasBanner"`
	RemovedRange       string   `json:"removedRange"`
	RemovedMin         *int     `json:"removedMin"`
	RemovedMax         *int     `json:"removedMax"`
	RemovedEstimate    float64  `json:"removedEstimate"`
	DeletionRatioPct   *float64 `json:"deletionRatioPct"`
	RealRatingAdjusted *float64 `json:"realRatingAdjusted"`
	RemovedText        string   `json:"removedText"`
	URL                string   `json:"url"`
	Address            string   `json:"address"`
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
	data := makeClientRows(rows)
	if err := os.MkdirAll(filepath.Dir(args.Output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(args.Output, []byte(makeHTML(data)), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", args.Output)
	return nil
}

func parseArgs(argv []string) (args, error) {
	out := args{Input: defaultInput, Output: defaultOutput}
	for i := 0; i < len(argv); i++ {
		key, value, consume := splitArg(argv, i)
		switch key {
		case "--input":
			out.Input = value
		case "--output":
			out.Output = value
		case "--help", "-h":
			fmt.Println(`Usage:
  go run ./cmd/dashboard
  go run ./cmd/dashboard --input output/places.json --output output/charts/nuernberg_dashboard.html`)
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
		if row.Status != "success" || row.Name == "" {
			continue
		}
		removedEstimate := 0.0
		if row.HasDefamationNotice {
			removedEstimate = mapsreview.RemovedSortValue(row)
		}
		lat := row.Lat
		lng := row.Lng
		if lat == nil || lng == nil {
			if coords := mapsreview.ExtractCoordinates(row.URL); coords != nil {
				lat = mapsreview.FloatPtr(coords.Lat)
				lng = mapsreview.FloatPtr(coords.Lng)
			}
		}
		out = append(out, clientRow{
			ID:                 row.ID,
			Name:               row.Name,
			Postcode:           mapsreview.StringValue(row.Postcode),
			Lat:                lat,
			Lng:                lng,
			Rating:             row.Rating,
			ReviewCount:        row.ReviewCount,
			Category:           mapsreview.StringValue(row.Category),
			HasBanner:          row.HasDefamationNotice,
			RemovedRange:       mapsreview.RemovedRange(row),
			RemovedMin:         row.RemovedMin,
			RemovedMax:         row.RemovedMax,
			RemovedEstimate:    removedEstimate,
			DeletionRatioPct:   row.DeletionRatioPct,
			RealRatingAdjusted: row.RealRatingAdjusted,
			RemovedText:        mapsreview.StringValue(row.RemovedText),
			URL:                row.URL,
			Address:            mapsreview.StringValue(row.Address),
		})
	}
	return out
}

func makeHTML(data []clientRow) string {
	postcodes := uniqueSorted(data, func(row clientRow) string { return row.Postcode })
	ranges := uniqueSorted(data, func(row clientRow) string { return row.RemovedRange })
	sort.SliceStable(ranges, func(i, j int) bool {
		return maxEstimateForRange(data, ranges[i]) > maxEstimateForRange(data, ranges[j])
	})
	jsonData, _ := json.Marshal(data)
	jsonText := strings.ReplaceAll(string(jsonData), "<", "\\u003c")

	postcodeOptions := ""
	for _, postcode := range postcodes {
		postcodeOptions += fmt.Sprintf(`<option value="%s">%s</option>`, escAttr(postcode), esc(postcode))
	}
	rangeOptions := ""
	for _, r := range ranges {
		if r != "" {
			rangeOptions += fmt.Sprintf(`<option value="%s">%s</option>`, escAttr(r), esc(r))
		}
	}

	page := `<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Nürnberg Google-Maps-Bewertungen Dashboard</title>
  <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css">
  <style>
    :root {
      color-scheme: light;
      --red: #cf2a1b;
      --red-dark: #9f2017;
      --text: #3f3f3f;
      --muted: #6f6f6f;
      --line: #d6d6d6;
      --soft: #f4f4f4;
      --blue: #1f6f8b;
      --green: #2d7b3f;
      --orange: #d97a1d;
      --shadow: 0 2px 10px rgba(0,0,0,.18);
      --sans: Arial, Helvetica, sans-serif;
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body { margin: 0; min-height: 100vh; font-family: var(--sans); color: var(--text); background: #fff; }
    .sitebar { height: 76px; background: #fff; border-bottom: 1px solid #e3e3e3; box-shadow: 0 1px 5px rgba(0,0,0,.18); }
    .sitebar-inner { position: relative; width: min(1320px, calc(100vw - 32px)); height: 100%; margin: 0 auto; display: flex; align-items: center; gap: 42px; }
    .menu-word { display: flex; align-items: center; gap: 14px; color: #666; font-size: 22px; font-weight: 700; text-transform: uppercase; }
    .hamburger { display: grid; gap: 6px; width: 44px; }
    .hamburger span { display: block; height: 5px; border-radius: 2px; background: #666; }
    .top-icons { display: flex; gap: 28px; color: #666; font-size: 33px; line-height: 1; }
    .n-logo { position: absolute; right: 0; top: 0; width: 170px; height: 128px; padding: 66px 14px 10px; background: var(--red); color: #fff; font-size: 24px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; z-index: 5; }
    .n-logo::before { content: ""; position: absolute; left: 14px; right: 14px; top: 58px; height: 2px; background: #fff; opacity: .9; }
    .n-logo::after { content: "⌂⌂"; position: absolute; right: 13px; top: 20px; color: #fff; font-size: 36px; letter-spacing: -12px; transform: scaleX(1.4); }
    .hero { min-height: 380px; margin: 0 0 30px; background: linear-gradient(180deg, rgba(38,120,155,.25), rgba(38,120,155,.25)), linear-gradient(135deg, #b7d5e1 0%, #e1edf2 45%, #bdc8cf 100%); display: flex; align-items: end; }
    .hero-inner { width: min(1320px, calc(100vw - 32px)); margin: 0 auto; padding: 140px 0 42px; }
    .hero-title { width: min(760px, 100%); padding: 24px 28px; background: rgba(55,55,55,.78); color: #fff; font-size: clamp(32px, 4vw, 52px); line-height: 1.12; font-weight: 400; }
    .hero-subtitle { width: min(760px, 100%); margin-top: 14px; padding: 18px 22px; background: #fff; border-radius: 5px; box-shadow: var(--shadow); color: #777; font-size: 20px; line-height: 1.45; }
    main { width: min(1320px, calc(100vw - 32px)); margin: 0 auto 70px; }
    .controls { position: sticky; top: 0; z-index: 2000; display: grid; grid-template-columns: minmax(280px, 1fr) 140px 160px 170px 150px auto; gap: 12px; align-items: end; padding: 16px; margin: 0 0 24px; background: #fff; border: 1px solid var(--line); box-shadow: 0 2px 8px rgba(0,0,0,.12); }
    label { display: block; margin-bottom: 6px; color: #666; font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; }
    input, select, button { font: inherit; }
    input, select { width: 100%; height: 44px; padding: 0 12px; border: 1px solid #cfcfcf; border-radius: 5px; background: #fff; color: #333; outline: none; }
    input:focus, select:focus { border-color: var(--blue); box-shadow: 0 0 0 3px rgba(31,111,139,.13); }
    .reset { height: 44px; border: 0; border-radius: 5px; padding: 0 18px; background: #333; color: #fff; font-weight: 700; cursor: pointer; }
    .grid { display: grid; gap: 16px; }
    .kpis { grid-template-columns: repeat(5, minmax(0, 1fr)); }
    .card { background: #fff; border: 1px solid var(--line); overflow: hidden; }
    .kpi { padding: 18px; border-top: 5px solid var(--red); }
    .kpi:nth-child(2) { border-top-color: var(--orange); }
    .kpi:nth-child(3) { border-top-color: var(--blue); }
    .kpi:nth-child(4) { border-top-color: var(--red-dark); }
    .kpi:nth-child(5) { border-top-color: var(--green); }
    .kpi .value { display: block; color: #333; font-size: clamp(30px, 3vw, 44px); font-weight: 400; line-height: 1; }
    .kpi .label { display: block; margin-top: 8px; color: var(--muted); font-size: 14px; }
    .panel-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); align-items: stretch; margin-top: 18px; }
    .panel { padding: 18px; min-height: 330px; }
    .panel h2, .dist h2 { margin: 0 0 8px; color: #333; font-size: 22px; font-weight: 700; }
    .panel p { margin: 0 0 16px; color: var(--muted); font-size: 13px; }
    .bars { display: grid; gap: 10px; }
    .bar-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 7px 10px; align-items: center; font-size: 12px; }
    .bar-link { width: 100%; padding: 0; border: 0; background: transparent; color: inherit; text-align: left; text-decoration: none; cursor: pointer; }
    .bar-link:hover .bar-name, .bar-link:focus .bar-name { color: var(--red); text-decoration: underline; }
    .bar-link:focus { outline: 2px solid rgba(207,42,27,.35); outline-offset: 3px; }
    .bar-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
    .bar-value { color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
    .track { grid-column: 1 / -1; height: 8px; background: #e7e7e7; overflow: hidden; }
    .fill { height: 100%; background: var(--red); }
    .fill.orange { background: var(--orange); }
    .fill.green { background: var(--green); }
    .dist { padding: 18px; margin-top: 18px; }
    .dist-row { display: grid; grid-template-columns: 130px minmax(0, 1fr) 90px; gap: 12px; align-items: center; margin: 8px 0; font-size: 13px; }
    .dist-track { height: 12px; background: #e7e7e7; overflow: hidden; }
    .dist-fill { height: 100%; background: var(--red); }
    .map-panel { margin-top: 18px; padding: 18px; }
    .map-panel h2 { margin: 0 0 8px; color: #333; font-size: 22px; font-weight: 700; }
    .map-panel p { margin: 0 0 16px; color: var(--muted); font-size: 13px; }
    #placesMap { position: relative; z-index: 0; height: 520px; border: 1px solid var(--line); background: #f4f4f4; }
    #placesMap.map-needs-key::after, #placesMap.map-active::after { position: absolute; left: 50%; top: 18px; z-index: 1000; transform: translateX(-50%); padding: 10px 14px; border-radius: 4px; background: rgba(51,51,51,.88); color: #fff; font-size: 13px; font-weight: 700; box-shadow: 0 2px 10px rgba(0,0,0,.22); pointer-events: none; }
    #placesMap.map-needs-key::after { content: "Strg/⌘ halten, um mit dem Mausrad zu zoomen"; }
    #placesMap.map-active::after { content: "Karten-Zoom aktiv"; }
    @media (pointer: coarse) { #placesMap.map-needs-key::after { content: "Zwei Finger zum Zoomen und Bewegen der Karte"; } }
    .map-empty { display: grid; place-items: center; height: 100%; padding: 20px; color: var(--muted); text-align: center; }
    .map-legend { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 10px; color: var(--muted); font-size: 13px; }
    .legend-dot { display: inline-block; width: 12px; height: 12px; margin-right: 6px; border-radius: 50%; vertical-align: -1px; }
    .tabs { display: flex; flex-wrap: wrap; gap: 8px; margin: 22px 0 14px; }
    .tab { border: 1px solid var(--line); border-radius: 5px; padding: 10px 14px; background: #f6f6f6; color: #333; font-weight: 700; cursor: pointer; }
    .tab.active { background: var(--red); border-color: var(--red); color: #fff; }
    .table-head { display: flex; justify-content: space-between; align-items: center; margin: 0 0 10px; color: var(--muted); font-size: 14px; }
    .table-head strong { color: #333; font-size: 22px; }
    .table-wrap { overflow: auto; background: #fff; border: 1px solid var(--line); }
    table { width: 100%; min-width: 1440px; border-collapse: collapse; table-layout: fixed; }
    col.rank { width: 70px; } col.name { width: 360px; } col.plz { width: 90px; } col.rating { width: 95px; } col.reviews { width: 125px; } col.banner { width: 100px; } col.removed { width: 120px; } col.estimate { width: 125px; } col.ratio { width: 120px; } col.real { width: 160px; } col.category { width: 175px; }
    th { position: sticky; top: 0; z-index: 2; padding: 0; background: #f3f3f3; border-bottom: 2px solid #cfcfcf; color: #333; font-size: 13px; text-align: left; }
    th button { display: flex; align-items: center; gap: 5px; width: 100%; min-height: 44px; padding: 12px; border: 0; background: transparent; color: inherit; font: inherit; font-weight: 700; text-align: inherit; cursor: pointer; }
    th.num button, th.rank button { justify-content: flex-end; text-align: right; }
    th button:hover { color: var(--red); background: #ececec; }
    .arrow { width: 1em; color: #777; }
    button.active .arrow { color: var(--red); }
    td { padding: 12px; border-bottom: 1px solid #e5e5e5; vertical-align: top; font-size: 14px; }
    tbody tr:nth-child(even) { background: #fafafa; }
    tbody tr:hover { background: #fff4f2; }
    tbody tr.target-row { background: #fff0cc; box-shadow: inset 5px 0 0 var(--red); }
    td.num, td.rank { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    td.name { overflow-wrap: anywhere; font-weight: 700; }
    .entry-address { display: block; margin-top: 4px; color: var(--muted); font-size: 12px; font-weight: 400; line-height: 1.35; }
    a { color: var(--red); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .pill { display: inline-flex; align-items: center; border-radius: 3px; padding: 3px 7px; background: #e8f2ea; color: var(--green); font-weight: 700; font-size: 12px; }
    .pill.bad { background: #fde6e2; color: var(--red); }
    footer { margin-top: 18px; color: var(--muted); font-size: 13px; }
    @media (max-width: 1200px) { .kpis, .panel-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .controls { grid-template-columns: 1fr 1fr 1fr; } .search { grid-column: 1 / -1; } .n-logo { position: static; height: 76px; width: 150px; margin-left: auto; padding-top: 48px; } .n-logo::before { top: 40px; } .n-logo::after { top: 4px; } }
    @media (max-width: 720px) { .sitebar-inner, main, .hero-inner { width: min(100vw - 20px, 1320px); } .top-icons { display: none; } .kpis, .panel-grid, .controls { grid-template-columns: 1fr; } .hero { min-height: 300px; } .hero-inner { padding-top: 92px; } .hero-title { font-size: 32px; padding: 18px; } .hero-subtitle { font-size: 16px; } }
  </style>
</head>
<body>
  <div class="sitebar" role="banner">
    <div class="sitebar-inner">
      <div class="menu-word"><span class="hamburger" aria-hidden="true"><span></span><span></span><span></span></span><span>Menü</span></div>
      <div class="top-icons" aria-hidden="true"><span>●</span><span>☝</span><span>▰</span></div>
      <div class="n-logo">Nürnberg</div>
    </div>
  </div>

  <section class="hero" aria-label="Seitentitel">
    <div class="hero-inner">
      <div class="hero-title">Nürnberg Google-Maps-Bewertungen</div>
      <div class="hero-subtitle">Interaktives Daten-Dashboard zu sichtbaren Hinweisen auf entfernte Bewertungen wegen Diffamierungsbeschwerden.</div>
    </div>
  </section>

  <main>
    <section class="controls" aria-label="Dashboard-Filter">
      <div class="control search"><label for="searchInput">Suche</label><input id="searchInput" type="search" placeholder="Name, PLZ, Kategorie, Löschbereich …" autocomplete="off"></div>
      <div class="control"><label for="postcodeFilter">PLZ</label><select id="postcodeFilter"><option value="">Alle PLZ</option>__POSTCODE_OPTIONS__</select></div>
      <div class="control"><label for="bannerFilter">Banner</label><select id="bannerFilter"><option value="all">Alle</option><option value="banner">Mit Banner</option><option value="clean">Ohne Banner</option></select></div>
      <div class="control"><label for="rangeFilter">Gelöscht</label><select id="rangeFilter"><option value="">Alle Bereiche</option>__RANGE_OPTIONS__</select></div>
      <div class="control"><label for="minReviews">Min. Rezensionen</label><input id="minReviews" type="number" min="0" step="1" value="0"></div>
      <button class="reset" id="resetFilters" type="button">Reset</button>
    </section>

    <section class="grid kpis" aria-label="Kennzahlen">
      <div class="card kpi"><span class="value" id="kpiPlaces">–</span><span class="label">Orte im Filter</span></div>
      <div class="card kpi"><span class="value" id="kpiBanners">–</span><span class="label">mit sichtbarem Banner</span></div>
      <div class="card kpi"><span class="value" id="kpiBannerPct">–</span><span class="label">Banner-Anteil</span></div>
      <div class="card kpi"><span class="value" id="kpiRemoved">–</span><span class="label">geschätzt entfernt</span></div>
      <div class="card kpi"><span class="value" id="kpiClean">–</span><span class="label">ohne sichtbaren Banner</span></div>
    </section>

    <section class="grid panel-grid" aria-label="Top-Rankings">
      <article class="card panel"><h2>Meiste entfernte Bewertungen</h2><p>Sortiert nach geschätztem Mittelpunkt.</p><div class="bars" id="barsRemoved"></div></article>
      <article class="card panel"><h2>Höchste Lösch-Quote</h2><p>Entfernte / sichtbare + entfernte Bewertungen.</p><div class="bars" id="barsRatio"></div></article>
      <article class="card panel"><h2>Schlechtestes Worst-Case-Rating</h2><p>Modell: alle entfernten Bewertungen waren 1★.</p><div class="bars" id="barsWorst"></div></article>
      <article class="card panel"><h2>Beste saubere Orte</h2><p>Ohne sichtbaren Banner, Rating zuerst.</p><div class="bars" id="barsClean"></div></article>
    </section>

    <section class="card dist" aria-label="Verteilung"><h2>Verteilung der Lösch-Stufen</h2><div id="distribution"></div></section>

    <section class="card map-panel" aria-label="Karte">
      <h2>Karte der erfassten Orte</h2>
      <p><span id="mapCount">–</span> Orte mit Koordinaten im aktuellen Filter. Marker anklicken, um den passenden Tabellenfilter zu wählen und den Eintrag zu markieren.</p>
      <div id="placesMap"><div class="map-empty">Karte wird geladen …</div></div>
      <div class="map-legend"><span><i class="legend-dot" style="background:#c9332c"></i>hohe Lösch-Quote</span><span><i class="legend-dot" style="background:#ef7d16"></i>sichtbarer Banner</span><span><i class="legend-dot" style="background:#2d7b3f"></i>kein sichtbarer Banner</span></div>
    </section>

    <nav class="tabs" aria-label="Tabellen-Presets">
      <button class="tab" data-mode="removed">Meiste entfernt</button>
      <button class="tab active" data-mode="ratio">Höchste Lösch-Quote</button>
      <button class="tab" data-mode="worst">Worst-Case-Rating</button>
      <button class="tab" data-mode="clean">Beste saubere Orte</button>
    </nav>

    <div class="table-head"><strong id="tableTitle">Höchste Lösch-Quote</strong><span id="resultCount">–</span></div>
    <section class="table-wrap" aria-label="Daten-Explorer">
      <table id="placesTable">
        <colgroup><col class="rank"><col class="name"><col class="plz"><col class="rating"><col class="reviews"><col class="banner"><col class="removed"><col class="estimate"><col class="ratio"><col class="real"><col class="category"></colgroup>
        <thead><tr>
          <th class="rank"><button data-sort="rank">Rang <span class="arrow"></span></button></th>
          <th><button data-sort="name">Name / Google Maps <span class="arrow"></span></button></th>
          <th><button data-sort="postcode">PLZ <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="rating">Rating <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="reviewCount">Rezensionen <span class="arrow"></span></button></th>
          <th><button data-sort="hasBanner">Banner <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="removedEstimate">Gelöscht <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="removedEstimate">Schätzwert <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="deletionRatioPct">Löschquote <span class="arrow"></span></button></th>
          <th class="num"><button data-sort="realRatingAdjusted">Worst-Case <span class="arrow"></span></button></th>
          <th><button data-sort="category">Kategorie <span class="arrow"></span></button></th>
        </tr></thead>
        <tbody></tbody>
      </table>
    </section>
    <footer>Quelle: Google Maps, öffentlich sichtbare Banner. „Kein Banner“ heißt nur: im Scrape war kein passender Hinweis sichtbar. Snapshot: __SNAPSHOT__.</footer>
  </main>

  <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
  <script id="placesData" type="application/json">__DATA__</script>
  <script>
    const DATA = JSON.parse(document.getElementById('placesData').textContent);
    const valid = DATA.filter(row => Number.isFinite(row.rating) && Number.isFinite(row.reviewCount));
    const fmt = new Intl.NumberFormat('de-DE');
    const fmt1 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 1, minimumFractionDigits: 1 });
    const fmt2 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2, minimumFractionDigits: 2 });
    const state = { mode: 'ratio', sortKey: 'deletionRatioPct', sortDir: 'desc' };
    const els = {
      search: document.getElementById('searchInput'), postcode: document.getElementById('postcodeFilter'), banner: document.getElementById('bannerFilter'), range: document.getElementById('rangeFilter'), minReviews: document.getElementById('minReviews'), reset: document.getElementById('resetFilters'), tbody: document.querySelector('#placesTable tbody'), resultCount: document.getElementById('resultCount'), tableTitle: document.getElementById('tableTitle'), mapCount: document.getElementById('mapCount')
    };
    const titles = { all: 'Alle Orte', removed: 'Meiste entfernte Bewertungen', ratio: 'Höchste Lösch-Quote', worst: 'Schlechtestes Worst-Case-Rating', clean: 'Beste saubere Orte' };
    let placesMap = null;
    let markerLayer = null;
    let mapUnavailable = false;
    let mapHintTimer = null;
    const activeMapKeys = new Set();

    function pct(value) { return Number.isFinite(value) ? fmt1.format(value) + '%' : '–'; }
    function rating(value, digits = 1) { return Number.isFinite(value) ? (digits === 2 ? fmt2.format(value) : fmt1.format(value)) : '–'; }
    function n(value) { return Number.isFinite(value) ? fmt.format(value) : '–'; }
    function esc(s) { return String(s ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch])); }
    function searchText(row) { return [row.name, row.postcode, row.category, row.removedRange, row.removedText, row.address].join(' ').toLowerCase(); }
    function matches(row) {
      const q = els.search.value.trim().toLowerCase();
      if (q && !searchText(row).includes(q)) return false;
      if (els.postcode.value && row.postcode !== els.postcode.value) return false;
      if (els.banner.value === 'banner' && !row.hasBanner) return false;
      if (els.banner.value === 'clean' && row.hasBanner) return false;
      if (els.range.value && row.removedRange !== els.range.value) return false;
      if (Number(row.reviewCount || 0) < Number(els.minReviews.value || 0)) return false;
      return true;
    }
    function filtered() { return valid.filter(matches); }
    function bannerRows(rows) { return rows.filter(row => row.hasBanner && Number.isFinite(row.removedEstimate)); }
    function cleanRows(rows) { return rows.filter(row => !row.hasBanner); }
    function defaultSortFor(mode) {
      if (mode === 'removed') return ['removedEstimate', 'desc'];
      if (mode === 'ratio') return ['deletionRatioPct', 'desc'];
      if (mode === 'worst') return ['realRatingAdjusted', 'asc'];
      if (mode === 'clean') return ['rating', 'desc'];
      return ['rank', 'asc'];
    }
    function modeRows(rows) {
      if (state.mode === 'removed') return bannerRows(rows);
      if (state.mode === 'ratio') return bannerRows(rows).filter(row => Number.isFinite(row.deletionRatioPct));
      if (state.mode === 'worst') return bannerRows(rows).filter(row => Number.isFinite(row.realRatingAdjusted));
      if (state.mode === 'clean') return cleanRows(rows);
      return rows;
    }
    function value(row, key, index) {
      if (key === 'rank') return index + 1;
      if (key === 'hasBanner') return row.hasBanner ? 1 : 0;
      const v = row[key];
      return typeof v === 'string' ? v.toLowerCase() : (Number.isFinite(v) ? v : -Infinity);
    }
    function sortRows(rows) {
      return rows.map((row, index) => ({ row, index })).sort((a, b) => {
        const av = value(a.row, state.sortKey, a.index);
        const bv = value(b.row, state.sortKey, b.index);
        const result = typeof av === 'string' ? av.localeCompare(bv, 'de') : av - bv;
        return state.sortDir === 'asc' ? result : -result;
      }).map(item => item.row);
    }
    function updateKpis(rows) {
      const banners = bannerRows(rows);
      const clean = cleanRows(rows);
      const removedSum = banners.reduce((sum, row) => sum + row.removedEstimate, 0);
      document.getElementById('kpiPlaces').textContent = n(rows.length);
      document.getElementById('kpiBanners').textContent = n(banners.length);
      document.getElementById('kpiBannerPct').textContent = pct((banners.length / Math.max(rows.length, 1)) * 100);
      document.getElementById('kpiRemoved').textContent = n(Math.round(removedSum));
      document.getElementById('kpiClean').textContent = n(clean.length);
    }
    function hasCoords(row) { return Number.isFinite(row.lat) && Number.isFinite(row.lng); }
    function markerColor(row) {
      if (!row.hasBanner) return '#2d7b3f';
      if ((row.deletionRatioPct || 0) >= 10) return '#c9332c';
      return '#ef7d16';
    }
    function markerMode(row) { return row.hasBanner ? 'ratio' : 'clean'; }
    function isMapModifier(event) { return event.ctrlKey || event.metaKey || event.altKey; }
    function setMapScrollMode(enabled) {
      if (!placesMap) return;
      const root = document.getElementById('placesMap');
      if (enabled) {
        placesMap.scrollWheelZoom.enable();
        root.classList.add('map-active');
        root.classList.remove('map-needs-key');
      } else {
        placesMap.scrollWheelZoom.disable();
        root.classList.remove('map-active');
      }
    }
    function flashMapHint() {
      const root = document.getElementById('placesMap');
      if (!root || root.classList.contains('map-active')) return;
      root.classList.add('map-needs-key');
      window.clearTimeout(mapHintTimer);
      mapHintTimer = window.setTimeout(() => root.classList.remove('map-needs-key'), 1300);
    }
    function setupMapGestureGate(root) {
      root.addEventListener('wheel', event => {
        if (isMapModifier(event)) {
          event.preventDefault();
          setMapScrollMode(true);
        } else {
          flashMapHint();
        }
      }, { capture: true, passive: false });
      root.addEventListener('touchstart', event => {
        if (event.touches && event.touches.length > 1) root.classList.add('map-active');
        else flashMapHint();
      }, { passive: true });
      root.addEventListener('touchend', () => root.classList.remove('map-active'), { passive: true });
      window.addEventListener('keydown', event => {
        if (['Control', 'Meta', 'Alt'].includes(event.key)) {
          activeMapKeys.add(event.key);
          setMapScrollMode(true);
        }
      });
      window.addEventListener('keyup', event => {
        if (['Control', 'Meta', 'Alt'].includes(event.key)) activeMapKeys.delete(event.key);
        if (activeMapKeys.size === 0) setMapScrollMode(false);
      });
      window.addEventListener('blur', () => {
        activeMapKeys.clear();
        setMapScrollMode(false);
      });
    }
    function initMap() {
      if (placesMap || mapUnavailable) return Boolean(placesMap);
      const root = document.getElementById('placesMap');
      if (typeof L === 'undefined') {
        mapUnavailable = true;
        root.innerHTML = '<div class="map-empty">Karte konnte nicht geladen werden. Internetzugriff auf Leaflet/OpenStreetMap prüfen.</div>';
        return false;
      }
      root.innerHTML = '';
      placesMap = L.map(root, { scrollWheelZoom: false, touchZoom: true }).setView([49.4521, 11.0767], 12);
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 19, attribution: '&copy; OpenStreetMap-Mitwirkende' }).addTo(placesMap);
      markerLayer = L.layerGroup().addTo(placesMap);
      setupMapGestureGate(root);
      return true;
    }
    function renderMap(rows) {
      if (!initMap()) return;
      markerLayer.clearLayers();
      const mapped = rows.filter(hasCoords);
      els.mapCount.textContent = n(mapped.length) + ' von ' + n(rows.length);
      for (const row of mapped) {
        const marker = L.circleMarker([row.lat, row.lng], { radius: row.hasBanner ? 7 : 5, color: '#fff', weight: 1.5, fillColor: markerColor(row), fillOpacity: .9 });
        marker.bindTooltip(row.name + (row.hasBanner ? ' · ' + row.removedRange : ''), { direction: 'top' });
        marker.on('click', () => {
          activateMode(markerMode(row));
          render();
          requestAnimationFrame(() => focusEntry(row.id));
        });
        marker.addTo(markerLayer);
      }
      if (mapped.length) {
        const bounds = L.latLngBounds(mapped.map(row => [row.lat, row.lng]));
        placesMap.fitBounds(bounds.pad(0.12), { maxZoom: 14, animate: false });
      } else {
        placesMap.setView([49.4521, 11.0767], 12);
      }
    }
    function renderBars(id, mode, rows, metric, label, color, maxValue) {
      const root = document.getElementById(id);
      const top = rows.slice(0, 8);
      const max = maxValue ?? Math.max(1, ...top.map(metric));
      root.innerHTML = top.map(row => '<a class="bar-row bar-link" href="#placesTable" data-mode="' + mode + '" data-entry-id="' + esc(row.id) + '"><div class="bar-name" title="' + esc(row.name) + '">' + esc(row.name) + '</div><div class="bar-value">' + label(row) + '</div><div class="track"><div class="fill ' + color + '" style="width:' + Math.max(2, Math.min(100, metric(row) / max * 100)) + '%"></div></div></a>').join('') || '<p>Keine Daten im Filter.</p>';
    }
    function renderDistribution(rows) {
      const banners = bannerRows(rows);
      const bins = ['über 250','201–250','151–200','101–150','51–100','21–50','11–20','6–10','2–5','1'];
      const counts = bins.map(bin => ({ bin, count: banners.filter(row => row.removedRange === bin).length })).filter(item => item.count > 0);
      const max = Math.max(1, ...counts.map(item => item.count));
      document.getElementById('distribution').innerHTML = counts.map(item => '<div class="dist-row"><strong>' + esc(item.bin) + '</strong><div class="dist-track"><div class="dist-fill" style="width:' + (item.count / max * 100) + '%"></div></div><span>' + n(item.count) + '</span></div>').join('') || '<p>Keine Banner im Filter.</p>';
    }
    function updatePanels(rows) {
      const banners = bannerRows(rows);
      renderBars('barsRemoved', 'removed', [...banners].sort((a,b) => b.removedEstimate - a.removedEstimate), row => row.removedEstimate, row => row.removedRange + ' · ' + n(row.removedEstimate), '', 300);
      renderBars('barsRatio', 'ratio', [...banners].sort((a,b) => (b.deletionRatioPct ?? -1) - (a.deletionRatioPct ?? -1)), row => row.deletionRatioPct || 0, row => pct(row.deletionRatioPct), '', Math.max(10, ...banners.map(row => row.deletionRatioPct || 0)));
      renderBars('barsWorst', 'worst', [...banners].filter(row => Number.isFinite(row.realRatingAdjusted)).sort((a,b) => a.realRatingAdjusted - b.realRatingAdjusted), row => 5 - row.realRatingAdjusted, row => rating(row.rating) + '★ → ' + rating(row.realRatingAdjusted, 2) + '★', 'orange', 4);
      renderBars('barsClean', 'clean', [...cleanRows(rows)].sort((a,b) => b.rating - a.rating || b.reviewCount - a.reviewCount), row => row.rating, row => rating(row.rating) + ' · ' + n(row.reviewCount), 'green', 5);
      renderDistribution(rows);
    }
    function renderTable(rows) {
      const scoped = modeRows(rows);
      const sorted = sortRows(scoped);
      els.resultCount.textContent = n(sorted.length) + ' von ' + n(rows.length) + ' Orten im aktuellen Filter';
      els.tableTitle.textContent = titles[state.mode];
      els.tbody.innerHTML = sorted.map((row, index) => '<tr data-entry-id="' + esc(row.id) + '"><td class="rank">' + (index + 1) + '</td><td class="name"><a href="' + esc(row.url) + '" target="_blank" rel="noopener noreferrer">' + esc(row.name) + '</a>' + (row.address ? '<span class="entry-address">' + esc(row.address) + '</span>' : '') + '</td><td>' + esc(row.postcode) + '</td><td class="num">' + rating(row.rating) + '</td><td class="num">' + n(row.reviewCount) + '</td><td>' + (row.hasBanner ? '<span class="pill bad">Banner</span>' : '<span class="pill">sauber</span>') + '</td><td class="num">' + (row.hasBanner ? esc(row.removedRange) : '–') + '</td><td class="num">' + (row.hasBanner ? rating(row.removedEstimate) : '–') + '</td><td class="num">' + pct(row.deletionRatioPct) + '</td><td class="num">' + rating(row.realRatingAdjusted, 2) + '</td><td>' + esc(row.category) + '</td></tr>').join('');
      document.querySelectorAll('th button[data-sort]').forEach(button => {
        const active = button.dataset.sort === state.sortKey;
        button.classList.toggle('active', active);
        button.querySelector('.arrow').textContent = active ? (state.sortDir === 'asc' ? '▲' : '▼') : '';
      });
    }
    function render() {
      const rows = filtered();
      updateKpis(rows);
      updatePanels(rows);
      renderMap(modeRows(rows));
      renderTable(rows);
    }
    function activateMode(mode) {
      state.mode = mode;
      [state.sortKey, state.sortDir] = defaultSortFor(mode);
      document.querySelectorAll('.tab').forEach(tab => tab.classList.toggle('active', tab.dataset.mode === mode));
    }
    function focusEntry(entryId) {
      const row = Array.from(els.tbody.rows).find(tr => tr.dataset.entryId === entryId);
      if (!row) return;
      document.querySelectorAll('tbody tr.target-row').forEach(tr => tr.classList.remove('target-row'));
      row.classList.add('target-row');
      row.scrollIntoView({ behavior: 'smooth', block: 'center' });
      window.setTimeout(() => row.classList.remove('target-row'), 2800);
    }
    document.querySelectorAll('.bars').forEach(root => root.addEventListener('click', event => {
      const link = event.target.closest('.bar-link');
      if (!link) return;
      event.preventDefault();
      activateMode(link.dataset.mode);
      render();
      requestAnimationFrame(() => focusEntry(link.dataset.entryId));
    }));
    document.querySelectorAll('.tab').forEach(button => button.addEventListener('click', () => {
      activateMode(button.dataset.mode);
      render();
    }));
    document.querySelectorAll('th button[data-sort]').forEach(button => button.addEventListener('click', () => {
      const next = button.dataset.sort;
      if (next === state.sortKey) state.sortDir = state.sortDir === 'asc' ? 'desc' : 'asc';
      else { state.sortKey = next; state.sortDir = ['name','postcode','category','rank'].includes(next) ? 'asc' : 'desc'; }
      renderTable(filtered());
    }));
    [els.search, els.postcode, els.banner, els.range, els.minReviews].forEach(input => input.addEventListener('input', render));
    els.reset.addEventListener('click', () => {
      els.search.value = ''; els.postcode.value = ''; els.banner.value = 'all'; els.range.value = ''; els.minReviews.value = 0;
      activateMode('ratio');
      render();
    });
    render();
  </script>
</body>
</html>`

	return strings.NewReplacer(
		"__POSTCODE_OPTIONS__", postcodeOptions,
		"__RANGE_OPTIONS__", rangeOptions,
		"__SNAPSHOT__", time.Now().Format("02.01.2006"),
		"__DATA__", jsonText,
	).Replace(page)
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

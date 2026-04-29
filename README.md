# Nürnberg Google-Bewertungen: Diffamierungs-Löschbanner

Reproducible local Go workflow to collect publicly visible Google Maps place metadata, detect review-removal notices like:

> „21 bis 50 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.“

…and generate Nürnberg summary charts plus an interactive dashboard.

## Important caveats

- This is for personal research / journalism only. Respect Google Maps terms and local law.
- The scraper records only what is publicly visible at scrape time.
- Missing banner ≠ definitely no removals. It only means no matching visible notice was detected.
- The adjusted rating assumes all removed reviews were 1-star. That is a worst-case model, not a fact.
- Use slow delays. If Google shows CAPTCHA, stop or solve manually in the headed browser.

## Setup

Requirements:

- Go 1.25+
- Chrome or Chromium available on your PATH / standard install location
- Optional for PNG export: ImageMagick `magick`

```bash
make setup
# or directly:
go mod download
```

## 1) Collect data

Full Nürnberg run:

```bash
make scrape ARGS="--postcodes all --headless=false"
```

Small test run:

```bash
make scrape ARGS="--postcodes 90402 --queries restaurant,café --max-results 20 --headless=false"
```

Outputs:

- `output/discovery.json` — discovered Google Maps places
- `output/places.json` — scraped data
- `output/places.csv` — spreadsheet-friendly export
- `output/metadata.json` — scrape settings, counts, timestamp, and user agent

Useful flags:

```bash
--postcodes 90402,90403
--queries restaurant,café,imbiss,pizzeria,bäckerei
--discovery-only
--scrape-only
--delay-min 4000 --delay-max 9000
--out output/places.json --csv output/places.csv
```

## 2) Improve data quality

Backfill missing addresses:

```bash
make backfill ARGS="--headless=true --concurrency 4"
```

Validate the scrape output:

```bash
make validate
go run ./cmd/validate --strict-nuremberg
```

Validation reports missing addresses, missing rating/review counts, non-Nürnberg postcodes, duplicate URLs/IDs, and banner rows with parse issues.

## 3) Generate charts and dashboard

```bash
make charts ARGS="--png"
make dashboard
```

Outputs:

- `output/charts/nuernberg_dashboard.html` — interactive app with KPIs, filters, map, sortable explorer table, and Google Maps links
- `output/charts/nuernberg_overall_summary.svg/.png`
- `output/charts/nuernberg_90402_summary.svg/.png`
- `output/charts/nuernberg_most_removed.csv`
- `output/charts/nuernberg_most_removed.md`
- `output/charts/nuernberg_most_removed.html`

If `magick` is not installed, `--png` skips PNG files and still writes SVGs.

The dashboard map uses Leaflet with OpenStreetMap tiles, so map tiles require internet access when opening the HTML file.

## Tests / checks

```bash
make test
make check
# or directly:
go test ./...
go run ./cmd/validate
```

## What the charts show

1. **Höchste Lösch-Quote**  
   `removed_midpoint / (visible_reviews + removed_midpoint)`

2. **Schlechtestes „echtes“ Rating**  
   Assumption: every removed review was 1-star.

3. **Beste „saubere“ Orte**  
   No visible defamation-removal banner, sorted by rating then review count.

4. **Verteilung der Lösch-Stufen**  
   Counts places by Google’s visible removal ranges.

## Nürnberg PLZ included by default

`90402, 90403, 90408, 90409, 90411, 90419, 90425, 90427, 90429, 90431, 90439, 90441, 90443, 90449, 90451, 90453, 90455, 90459, 90461, 90469, 90471, 90473, 90475, 90478, 90480, 90482, 90489, 90491`

## Notes on completeness

Google Maps search is not a complete database export. For better coverage, run multiple query types per PLZ and dedupe results. The defaults are:

`restaurant, café, imbiss, pizzeria, bäckerei`

If you want a stricter “Restaurants only” dataset, use only `--queries restaurant` and manually filter `output/places.csv`.

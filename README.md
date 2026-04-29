# Nürnberg Google-Bewertungen: Diffamierungs-Löschbanner

Reproducible local workflow to create charts like the Düsseldorf Reddit post, but for Nürnberg.

It collects publicly visible Google Maps place metadata and detects notices like:

> „21 bis 50 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.“

Then it generates overview charts per PLZ and for the whole city.

## Important caveats

- This is for personal research / journalism only. Respect Google Maps terms and local law.
- The script records only what is publicly visible at scrape time.
- Missing banner ≠ definitely no removals. It only means no matching visible notice was detected.
- The adjusted rating assumes all removed reviews were 1-star. That is a worst-case model, not a fact.
- Use slow delays. If Google shows CAPTCHA, stop or solve manually in the headed browser.

## Setup

```bash
npm run setup
```

This installs Playwright and its Chromium browser.

## 1) Collect data

Full Nürnberg run:

```bash
npm run scrape -- --postcodes all --headless false
```

Small test run:

```bash
npm run scrape -- --postcodes 90402 --queries restaurant,café --max-results 20 --headless false
```

Outputs:

- `output/discovery.json` — discovered Google Maps places
- `output/places.json` — scraped data
- `output/places.csv` — spreadsheet-friendly export

Useful flags:

```bash
--postcodes 90402,90403
--queries restaurant,café,imbiss,pizzeria,bäckerei
--discovery-only
--scrape-only
--delay-min 4000 --delay-max 9000
```

## 2) Generate charts and dashboard

```bash
npm run charts -- --png
npm run dashboard
```

Outputs:

- `output/charts/nuernberg_dashboard.html` — combined interactive app with KPIs, rankings, filters, sortable explorer table, and Google Maps links
- `output/charts/nuernberg_overall_summary.svg/.png`
- `output/charts/nuernberg_90402_summary.svg/.png`
- `output/charts/nuernberg_most_removed.csv` — all banner places sorted by estimated removed count
- `output/charts/nuernberg_most_removed.md` — Markdown version of the same ranking
- `output/charts/nuernberg_most_removed.html` — browser table with names linked to Google Maps
- etc.

## What the charts show

1. **Höchste Lösch-Quote**  
   `removed_midpoint / (visible_reviews + removed_midpoint)`

2. **Schlechtestes „echtes“ Rating**  
   Assumption: every removed review was 1-star.

3. **Beste „saubere“ Orte**  
   No visible defamation-removal banner, sorted by rating then review count.

4. **Verteilung der Lösch-Stufen**  
   Counts places by Google’s visible removal ranges.

## Nürnberg PLZ included

`90402, 90403, 90408, 90409, 90411, 90419, 90425, 90427, 90429, 90431, 90439, 90441, 90443, 90449, 90451, 90453, 90455, 90459, 90461, 90469, 90471, 90473, 90475, 90478, 90480, 90482, 90489, 90491`

## Notes on completeness

Google Maps search is not a complete database export. For better coverage, run multiple query types per PLZ and dedupe results. The defaults are:

`restaurant, café, imbiss, pizzeria, bäckerei`

If you want a stricter “Restaurants only” dataset, use only `--queries restaurant` and manually filter `output/places.csv`.

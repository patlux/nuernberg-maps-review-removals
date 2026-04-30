# Nürnberg Google-Bewertungen: Diffamierungs-Löschbanner

Reproduzierbarer lokaler Go-Workflow, um öffentlich sichtbare Google-Maps-Ortsdaten zu sammeln, Hinweise auf entfernte Bewertungen zu erkennen, zum Beispiel:

> „21 bis 50 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.“

…und daraus Nürnberg-Auswertungen sowie ein interaktives Dashboard zu erzeugen.

## Wichtige Hinweise

- Nur für private Recherche / Journalismus gedacht. Google-Maps-Bedingungen und geltendes Recht beachten.
- Der Scraper speichert nur, was zum Scrape-Zeitpunkt öffentlich sichtbar ist. Manuell geprüfte Abweichungen können als Overrides in `internal/mapsreview/data/place_overrides.json` gepflegt werden.
- Kein Banner ≠ definitiv keine entfernten Bewertungen. Es bedeutet nur: Beim Scrape wurde kein passender sichtbarer Hinweis erkannt.
- Das angepasste Rating nimmt an, dass alle entfernten Bewertungen 1-Stern-Bewertungen waren. Das ist ein Worst-Case-Modell, keine Tatsache.
- Langsame Delays verwenden. Wenn Google ein CAPTCHA zeigt: stoppen oder im sichtbaren Browser manuell lösen.

## Einrichtung

Voraussetzungen:

- Go 1.25+
- Chrome oder Chromium im `PATH` oder an einem Standard-Installationsort
- Optional für PNG-Export: ImageMagick `magick`

```bash
make setup
# oder direkt:
go mod download
```

## 1) Daten sammeln

Vollständiger Nürnberg-Lauf:

```bash
make scrape ARGS="--postcodes all --headless=false"
```

Kleiner Testlauf:

```bash
make scrape ARGS="--postcodes 90402 --queries restaurant,café --max-results 20 --headless=false"
```

Ausgaben:

- `output/discovery.json` — gefundene Google-Maps-Orte
- `output/places.json` — gescrapte Daten inklusive Koordinaten und, sofern zuordenbar, `bezirkId` / `bezirkName`
- `output/places.csv` — CSV-Export für Tabellenkalkulationen
- `output/metadata.json` — Scrape-Einstellungen, Zählwerte, Zeitstempel und User-Agent

Nützliche Optionen:

```bash
--postcodes 90402,90403
--queries restaurant,café,imbiss,pizzeria,bäckerei
--discovery-only
--scrape-only
--scrape-only --rescrape-all   # alle gefundenen Orte erneut lesen, auch bereits erfolgreiche
--scrape-only --rescrape-all --allow-banner-clears   # zuvor erkannte Banner nach manueller Prüfung entfernen lassen
--scrape-only --rescrape-all --resume-from 1288   # vollständigen Rescan an 1-basierter Todo-Position fortsetzen
--scrape-only --rescrape-all --resume-from 1288 --scrape-limit 200   # sichereren Teil-Scan ausführen
--delay-min 4000 --delay-max 9000
--out output/places.json --csv output/places.csv
```

## 2) Datenqualität verbessern

Fehlende Adressen nachtragen:

```bash
make backfill ARGS="--headless=true --concurrency 4"
```

Scrape-Ergebnis validieren:

```bash
make validate
go run ./cmd/validate --strict-nuremberg
```

Die Validierung meldet fehlende Adressen, fehlende Ratings/Rezensionszahlen, fehlende Nürnberg-Bezirkszuordnungen, Nicht-Nürnberger Postleitzahlen, doppelte URLs/IDs und Banner-Zeilen mit Parse-Problemen.

## 3) Diagramme und Dashboard erzeugen

```bash
make charts ARGS="--png"
make dashboard
```

Ausgaben:

- `output/charts/nuernberg_dashboard.html` — interaktive App mit KPIs, Filtern, Karte, sortierbarer Explorer-Tabelle und Google-Maps-Links
- `output/charts/nuernberg_overall_summary.svg/.png`
- `output/charts/nuernberg_90402_summary.svg/.png`
- `output/charts/nuernberg_most_removed.csv`
- `output/charts/nuernberg_most_removed.md`
- `output/charts/nuernberg_most_removed.html`

Wenn `magick` nicht installiert ist, überspringt `--png` die PNG-Dateien und schreibt weiterhin SVGs.

Die erzeugten Diagramm- und Dashboard-Dateien unter `output/charts/` werden von git ignoriert. Im Repository bleiben nur die Scrape-Snapshots (`output/places.json`, `output/places.csv`, `output/metadata.json`, optional `output/discovery.json`) versioniert; `make site` baut daraus `public/` für GitHub Pages neu.

Die Dashboard-Karte nutzt Leaflet mit CARTO-Kartenkacheln auf Basis von OpenStreetMap-Daten. Beim Öffnen der HTML-Datei ist deshalb Internetzugriff für Kartenkacheln nötig. Das Dashboard gruppiert, filtert und überlagert Einträge außerdem nach Nürnberger statistischem Bezirk (`Bezirk`).

## Veröffentlichung mit GitHub Pages

GitHub Pages ist auf den Branch `gh-pages` konfiguriert. Der Branch enthält nur das generierte `public/`-Artefakt; die Quell- und Snapshot-Dateien bleiben auf `main`.

Öffentliche URL: <https://nuernberg-maps-review-removals.patwoz.dev/>

Lokale Vorschau des Veröffentlichungs-Artefakts:

```bash
make site
python3 -m http.server --directory public 8080
```

Veröffentlichen:

```bash
make deploy-pages
```

Im GitHub-Repository muss dafür **Settings → Pages → Source: Deploy from a branch**, Branch `gh-pages`, Ordner `/` aktiv sein.

## GitHub Actions

Der Workflow `.github/workflows/refresh-and-deploy.yml` baut und veröffentlicht GitHub Pages bei jedem Push auf `main` neu.

Ein Daten-Refresh läuft bewusst nur manuell über **Actions → Refresh data and deploy site → Run workflow** mit aktivierter Option `refresh_data`. Standardmäßig wird dann der vorhandene Discovery-Snapshot komplett neu gescrapt:

```bash
--scrape-only --rescrape-all --save-every 25 --delay-min 4000 --delay-max 9000 --headless=true
```

Falls Google ein CAPTCHA oder eine eingeschränkte Ansicht ausliefert, kann der Action-Lauf fehlschlagen oder unvollständige Daten liefern; dann lokal mit sichtbarem Browser neu laufen lassen. Zuvor erkannte Löschbanner werden bei automatischen Re-Scrapes standardmäßig nicht entfernt; dafür ist nach manueller Prüfung `--allow-banner-clears` nötig.

## Tests / Checks

```bash
make test
make check
# oder direkt:
go test ./...
go run ./cmd/validate
```

## Was die Diagramme zeigen

1. **Höchste Lösch-Quote**  
   `removed_midpoint / (visible_reviews + removed_midpoint)`

2. **Schlechtestes „echtes“ Rating**  
   Annahme: Jede entfernte Bewertung war eine 1-Stern-Bewertung.

3. **Beste Orte ohne Löschbanner**  
   Ohne sichtbaren Diffamierungs-Löschbanner, sortiert nach Rating und danach Rezensionszahl.

4. **Verteilung der Lösch-Stufen**  
   Zählt Orte nach Googles sichtbaren Löschbereichen.

## Nürnberger statistische Bezirke

Einträge mit Koordinaten werden über die offizielle Bezirksatlas-Geometrie von `online-service2.nuernberg.de/geoinf/ia_bezirksatlas/` den Nürnberger statistischen Bezirken zugeordnet. Die Geometrie liegt in `internal/mapsreview/data/nuernberg_statistische_bezirke.json`.

Punkte in nicht bewohnten Lücken dieser Quelle werden nur dann dem nächstgelegenen statistischen Bezirk zugeordnet, wenn die Zeile eine Nürnberger Postleitzahl hat. Nicht-Nürnberger Postleitzahlen bleiben ohne Bezirkszuordnung.

## Standardmäßig enthaltene Nürnberger PLZ

`90402, 90403, 90408, 90409, 90411, 90419, 90425, 90427, 90429, 90431, 90439, 90441, 90443, 90449, 90451, 90453, 90455, 90459, 90461, 90469, 90471, 90473, 90475, 90478, 90480, 90482, 90489, 90491`

## Hinweise zur Vollständigkeit

Die Google-Maps-Suche ist kein vollständiger Datenbankexport. Für bessere Abdeckung mehrere Suchbegriffe pro PLZ verwenden und Ergebnisse deduplizieren. Die Standard-Suchbegriffe sind:

`restaurant, café, imbiss, pizzeria, bäckerei, döner, burger, sushi, schnitzel, frühstück, brunch`

Für einen strengeren „nur Restaurants“-Datensatz nur `--queries restaurant` verwenden und `output/places.csv` anschließend manuell filtern.

## Lizenz

MIT, siehe [`LICENSE`](LICENSE).

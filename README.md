# Nürnberg Google-Bewertungen: Diffamierungs-Löschbanner

Reproduzierbarer lokaler Go-Workflow, um öffentlich sichtbare Google-Maps-Ortsdaten zu sammeln, Hinweise auf entfernte Bewertungen zu erkennen, zum Beispiel:

> „21 bis 50 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.“

…und daraus Nürnberg-Auswertungen sowie ein interaktives Dashboard zu erzeugen.

## Wichtige Hinweise

- Nur für private Recherche / Journalismus gedacht. Google-Maps-Bedingungen und geltendes Recht beachten.
- Der Scraper speichert nur, was zum Scrape-Zeitpunkt öffentlich sichtbar ist. Manuell geprüfte Abweichungen können als Overrides in `internal/mapsreview/data/place_overrides.json` gepflegt werden.
- Kein Banner ≠ definitiv keine entfernten Bewertungen. Es bedeutet nur: Beim Scrape wurde kein passender sichtbarer Hinweis erkannt.
- Das Dashboard zeigt ein ausdrücklich als Modell gekennzeichnetes **1-Stern-Szenario**, kein tatsächliches oder „wahres“ Rating und keine gesicherte Untergrenze. Sterne und genaue Anzahl entfernter Rezensionen sind unbekannt. Separate Diagramme und Exporte bleiben ohne synthetische Sterne-Ratings. Ältere Quelldaten können das frühere Modellfeld noch enthalten; sie werden nicht nachträglich verändert.
- Langsame Delays verwenden. Wenn Google ein CAPTCHA zeigt: stoppen oder im sichtbaren Browser manuell lösen.

## Einrichtung

Voraussetzungen:

- Go 1.25+
- Node.js 22+ für die browserfreien Dashboard-Regressionstests
- Chrome oder Chromium im `PATH` oder an einem Standard-Installationsort
- Optional als experimentelles CDP-Backend: Lightpanda
- Optional für Places-API-Discovery: Google Places API (New)-API-Key in `config.toml`
- Optional für PNG-Export: ImageMagick `magick` oder `convert`

```bash
make setup
# oder direkt:
go mod download
```

Für die optionale Places-API-Discovery `config.toml` aus `.config.toml.example` anlegen und `[places_api].api_key` setzen. `config.toml` bleibt lokal/git-ignoriert.

## 1) Daten sammeln

Standardmäßig nutzt der Scraper Chrome. Er liest die normale Google-Maps-Seite für Metadaten und die direkte Rezensionen-URL für Rating, Rezensionszahl und Löschbanner, weil die normale Maps-Ansicht Löschbanner teils nicht im DOM enthält.

Der Workflow ist immer zweistufig:

1. **Discovery** schreibt/erweitert `output/discovery.json`.
2. **Scrape** liest `output/discovery.json`, öffnet die Orte in Google Maps im Browser und schreibt `output/places.json` / `output/places.csv`.

Die Löschbanner-Erkennung passiert in beiden Varianten im Browser auf Google Maps. Die Places API wird nur optional für Discovery verwendet.

### Variante A: Discovery ohne Places API

Diese Variante nutzt nur den Browser: Google-Maps-Suchen werden geöffnet, sichtbare Ergebnislinks gesammelt und danach gescrapt.

```bash
# 1. Orte über Google-Maps-Suchergebnisse finden
make scrape ARGS="--discovery-only --postcodes all --headless=false"

# 2. Gefundene Orte im Browser scrapen, inklusive Rezensionen/Löschbanner
make scrape ARGS="--scrape-only --headless=false"
```

Vorteile: kein API-Key, keine Google-Cloud-Quota, kein API-Billing-Risiko. Nachteile: langsamer, stärker abhängig von der Google-Maps-Oberfläche und der sichtbaren Ergebnisliste.

### Variante B: Discovery mit Places API

Diese Variante nutzt die offizielle Places API (New) nur für die Ortssuche. Die Text-Search-Anfrage ist bewusst auf ID-only-Felder beschränkt:

```text
places.id,nextPageToken
```

Danach ist der Ablauf identisch: Die gefundenen `ChIJ...`-Place-IDs werden als Google-Maps-URLs in `output/discovery.json` gespeichert und im Browser gescrapt. Beim Scrape löst Google Maps die URL auf eine kanonische `/maps/place/.../data=...` URL auf; diese wird anschließend in `output/places.json` gespeichert, damit spätere Läufe direktere Maps-URLs/IDs haben.

```bash
# 1. Orte über Places API Text Search finden
make scrape ARGS="--places-api-discovery --discovery-only --places-api-pages 1"

# 2. Gefundene Orte im Browser scrapen, inklusive Rezensionen/Löschbanner
make scrape ARGS="--scrape-only --headless=false"
```

Für tiefere Discovery, wenn die Tagesquota entsprechend gesetzt ist:

```bash
make scrape ARGS="--places-api-discovery --discovery-only --places-api-pages 2"
make scrape ARGS="--scrape-only --headless=false"
```

Vorteile: bessere und stabilere Discovery-Abdeckung. Nachteile: API-Key, Quota-Management und Billing-Monitoring nötig. Die API liefert keine Löschbanner; dafür bleibt immer der Browser-Scrape nötig.

Vollständiger Nürnberg-Lauf mit der Standard-Browser-Discovery:

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
--city Nürnberg
--postcodes 90402,90403
--queries restaurant,café,imbiss,pizzeria,bäckerei
--discovery-only
--places-api-discovery --discovery-only   # experimentell: offizielle Places API Text Search, ID-only/no-cost-SKU laut Google-Preisliste; liest [places_api].api_key aus config.toml
--places-api-pages 1                       # API-Seiten pro PLZ/Suche; Default 1 hält die Standardsuchen unter 1.000 Requests/Tag
--scrape-only
--scrape-only --rescrape-all   # alle gefundenen Orte erneut lesen, auch bereits erfolgreiche
--scrape-only --rescrape-all --allow-banner-clears   # zuvor erkannte Banner nach manueller Prüfung entfernen lassen
--scrape-only --banner-audit-only --notice-attempts 2   # no-banner-Zeilen gezielt auf übersehene Banner prüfen; schreibt nur neu gefundene Banner
--scrape-only --rescrape-all --resume-from 1288   # vollständigen Rescan an 1-basierter Todo-Position fortsetzen
--scrape-only --rescrape-all --resume-from 1288 --scrape-limit 200   # sichereren Teil-Scan ausführen
--delay-min 4000 --delay-max 9000
--out output/places.json --csv output/places.csv
--discovery output/discovery.json --metadata output/metadata.json
```

### Andere Städte mit `--city`

`--city` ist für Scrape, Diagramme und Dashboard verfügbar. Es setzt den Ortsnamen für die Google-Maps-Suche, die Dashboard-Texte/SEO-Metadaten und die Diagramm-Dateipräfixe.

Wichtig: Für jede Nicht-Nürnberg-Stadt muss `--postcodes` explizit als CSV übergeben werden; `--postcodes all` ist dort absichtlich ungültig, damit nicht versehentlich die Nürnberger Standard-PLZ genutzt werden.

```bash
# Beispiel: Fürth-Orte suchen und scrapen
make scrape ARGS="--city Fürth --postcodes 90762,90763 --queries restaurant,café --headless=false"

# Diagramme mit City-Slug als Dateipräfix erzeugen, z. B. fuerth_overall_summary.svg
make charts ARGS="--city Fürth --png"

# Dashboard-Texte und Metadaten auf Fürth setzen
go run ./cmd/dashboard --city Fürth --output output/charts/fuerth_dashboard.html
```

Einschränkungen: Die statistischen Bezirke und Kartenflächen sind Nürnberg-spezifisch. Bei anderen Städten zeigt das Dashboard keine Nürnberger Bezirksflächen; `go run ./cmd/validate --strict-nuremberg` ist dann nicht passend. Für eigene Stadt-Domains `city`, `output`, `site_domain`, `site_url` und `site_output` in `config.toml` anpassen.

Optional kann der Scraper über CDP gegen einen bereits laufenden Browser wie Lightpanda laufen. Das ist experimentell; Chrome bleibt der Standard und war in Stichproben schneller:

```bash
LIGHTPANDA_DISABLE_TELEMETRY=true lightpanda serve --host 127.0.0.1 --port 9333
make scrape ARGS="--scrape-only --rescrape-all --cdp-url ws://127.0.0.1:9333 --save-every 25 --delay-min 4000 --delay-max 9000"
```

Lightpanda ist als Vergleichs- oder Fallback-Backend nützlich, sollte aber mit Stichproben gegen Chrome geprüft werden, bevor seine Ergebnisse übernommen werden.

Nach einem Refresh kann ein konservativer Banner-Audit helfen, zuvor übersehene Löschbanner zu finden. Der Audit prüft nur bestehende erfolgreiche Zeilen ohne Banner und schreibt ausschließlich neu gefundene Banner; bestehende Banner werden dabei nie entfernt:

```bash
make scrape ARGS="--scrape-only --banner-audit-only --notice-attempts 2 --save-every 25 --delay-min 4000 --delay-max 9000"
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

### Dashboard-Konfiguration

`cmd/dashboard` benötigt einen CARTO-Basemaps-Key in `[carto].api_key`. Ohne Key bricht der Dashboard-/Site-Build vor dem Schreiben von HTML ab.

`cmd/dashboard` lädt dafür eine lokale TOML-Konfiguration. Wenn `config.toml` im Projektroot existiert, wird sie automatisch verwendet; alternativ kann ein Pfad explizit übergeben werden:

```bash
cp .config.toml.example config.toml
$EDITOR config.toml

go run ./cmd/dashboard --config config.toml
# oder implizit, wenn config.toml existiert:
go run ./cmd/dashboard
```

`config.toml` ist git-ignoriert, weil darin rechtliche Kontakt-/Adressdaten stehen können. CLI-Flags überschreiben Werte aus der Konfiguration.

Beispiel:

```toml
city = "Nürnberg"
input = "output/places.json"
output = "output/charts/nuernberg_dashboard.html"
site_domain = "nuernberg-maps-review-removals.patwoz.dev"
site_url = "https://nuernberg-maps-review-removals.patwoz.dev"
site_output = "public"

[legal]
enabled = true
name = "Patrick Wozniak"
email = "hi@example.com"
address_lines = [
  "c/o Example GmbH",
  "Example Street 1",
  "90443 Nürnberg",
  "Deutschland",
]
note = "Example GmbH ist nicht Betreiberin dieses Angebots. Sie dient ausschließlich als Zustellanschrift."
post_handler = "Example GmbH"

[analytics]
# Standardmäßig aus. Vor Aktivierung Empfänger, Hosting, Aufbewahrung und
# Drittlandgrundlage prüfen und in privacy_text dokumentieren.
# Zusätzlich ist die Einwilligung des Besuchers nötig.
enabled = false
privacy_text = ""
src = "https://a.patwoz.dev/js/script.js"
domain = "nuernberg-maps-review-removals.patwoz.dev"

[carto]
# Externe Kacheln bleiben aus, bis die Datenschutzangaben geprüft sind.
enabled = false
privacy_text = ""
# Erforderlich für Dashboard/Site. Veröffentlicht im erzeugten HTML.
# Auf die eigene Website beschränken: https://carto.com/basemaps/apikey
api_key = ""

[places_api]
# Optional. Nur für --places-api-discovery erforderlich.
api_key = ""
```

Der CARTO-Key ist ein Browser-Key und steht zwangsläufig im erzeugten HTML sowie in den Kachelanfragen. Beschränke ihn im CARTO-Dashboard auf die Produktionsdomain. Für lokale HTTP-Vorschauen verlangt CARTO einen separaten Key mit `localhost`/`127.0.0.1` als erlaubten Hosts. `file://` sendet keinen passenden Referer für Websitebeschränkungen. Ein Google-Places-Key ersetzt den CARTO-Key nicht und wird nicht ins HTML übernommen.

Wenn `[legal]` fehlt oder `enabled = false` gesetzt ist, erzeugt das Dashboard keine `methodik.html`, `korrektur.html`, `impressum.html` und `datenschutz.html` und verlinkt diese Seiten nicht. Sobald Legal-Seiten aktiviert sind, müssen `name`, `email` und `address_lines` gesetzt sein.

Rechtlicher Hinweis: Die erzeugten Impressum-/Datenschutz-/Methodik-Texte sind technische Vorlagen und keine Rechtsberatung. Prüfe vor Veröffentlichung, ob Verantwortlicher, ladungsfähige Anschrift, E-Mail, Zustellanschrift, Hosting, Analytics, Kartenanbieter und sonstige Dienste für dein konkretes Setup korrekt und vollständig beschrieben sind. Eine c/o- oder Zustellanschrift sollte nur verwendet werden, wenn dort rechtliche Post zuverlässig entgegengenommen und an dich weitergeleitet wird.

Ausgaben:

- `output/charts/nuernberg_dashboard.html` — interaktive App mit KPIs, Filtern, Karte, sortierbarer Explorer-Tabelle und Google-Maps-Links
- `output/charts/nuernberg_overall_summary.svg/.png`
- `output/charts/nuernberg_90402_summary.svg/.png`
- `output/charts/nuernberg_most_removed.csv`
- `output/charts/nuernberg_most_removed.md`
- `output/charts/nuernberg_most_removed.html`

Ein ausdrücklich angeforderter PNG-Export bricht bei einem Exportfehler ab, statt einen erfolgreichen Build mit alten PNG-Dateien vorzutäuschen. `make site` benötigt keine PNG-Exporte.

Die erzeugten Diagramm- und Dashboard-Dateien unter `output/charts/` werden von git ignoriert. Im Repository bleiben nur die Scrape-Snapshots (`output/places.json`, `output/places.csv`, `output/metadata.json`, optional `output/discovery.json`) versioniert; `make site` ruft `cmd/dashboard --site` auf und baut daraus `public/` für GitHub Pages. `site_domain`, `site_url` und `site_output` kommen dabei ausschließlich aus `config.toml`.

Leaflet 1.9.4 und seine BSD-Lizenz sind eingebettet. Die Karte zeigt nach Aktivierung Orte und Bezirksflächen ohne Kontakt zu UNPKG oder Kartenanbietern. Externe CARTO-Kacheln bleiben standardmäßig deaktiviert. Für `[carto].enabled = true` sind aktivierte Legal-Seiten und vollständige, geprüfte `privacy_text`-Angaben nötig: verantwortlicher Anbieter und Empfänger, Datenarten, Zweck, Aufbewahrung und gegebenenfalls Drittlandgrundlage. Ein nichtleerer Text ist nur eine technische Voraussetzung, keine juristische Validierung. Besucher aktivieren die Karte separat und können sie wieder deaktivieren. Kachelanfragen verraten den betrachteten Ausschnitt, gegebenenfalls den ungefähren Standort.

## Veröffentlichung mit GitHub Pages

GitHub Pages ist auf den Branch `gh-pages` konfiguriert. Der Branch enthält nur das generierte `public/`-Artefakt; die Quell- und Snapshot-Dateien bleiben auf `main`.

Öffentliche URL: <https://nuernberg-maps-review-removals.patwoz.dev/>

Lokale Vorschau des Veröffentlichungs-Artefakts:

```bash
make site
python3 -m http.server --bind 127.0.0.1 --directory public 8080
```

Plausible ist auch bei vorhandener alter `src`/`domain`-Konfiguration standardmäßig aus. Eine Aktivierung braucht `[analytics].enabled = true`, Legal-Seiten und geprüfte `privacy_text`-Angaben zu tatsächlichem Betreiber/Hosting, Empfängern, Datenarten, Zweck, Aufbewahrung und Übermittlungsgrundlage. Erst nach zusätzlicher Einwilligung im Footer wird das Skript geladen. Die Einwilligung gilt pro Tab; der Widerruf lädt die Seite ohne Analytics neu. Ein einzelnes Ereignis kann auch bei rein aggregierten Berichten verarbeitet werden.

Für GitHub Actions muss die Dashboard-Konfiguration einschließlich `[carto].api_key` als Repository-Secret `DASHBOARD_CONFIG_TOML` hinterlegt werden. Ein optionaler `[places_api]`-Key ist nur für API-Discovery erforderlich und gehört nicht in die reine Deployment-Konfiguration.

Der Workflow schreibt daraus bei jedem Lauf eine private temporäre `config.toml` (Dateirechte `0600`). Die Konfigurationsdatei bleibt außerhalb des Repositorys. Nur die für die Website bestimmten Werte, einschließlich des CARTO-Browser-Keys, gelangen in das erzeugte HTML und damit nach `gh-pages`. Fehlt der CARTO-Key, stoppt der Build vor der Veröffentlichung. Eine Repository-Variable gleichen Namens bleibt für Konfigurationen ohne vertrauliche Daten unterstützt.

Bei einer Key-Rotation müssen die lokale `[carto].api_key`-Einstellung und das Secret `DASHBOARD_CONFIG_TOML` aktualisiert werden. Der nächste Deployment-Build verwendet dann automatisch den neuen Key für helle und dunkle Kartenkacheln.

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
node --test cmd/dashboard/*.test.cjs
go run ./cmd/validate
```

## Was die Diagramme zeigen

1. **Beobachtete Spannen**: Google-Spannen, bei gleicher Spanne alphabetisch. Die Balken verwenden Spannenmittelpunkte und für über 250 den frei gewählten Rechenwert 300.
2. **Hinweise nach Kategorie**: Anzahl erfasster Orte mit Hinweis; keine Bewertung einzelner Betriebe.
3. **Ohne beobachteten Hinweis**: nach angezeigtem Google-Rating und Rezensionszahl, ab konfigurierter Mindestzahl. Kein Hinweis ist kein Beleg für fehlende Entfernungen.
4. **Verteilung der Google-Spannen**: Häufigkeiten in der erfassten Stichprobe.

Jede Grafik enthält den Datenzeitraum aus den gespeicherten Abrufzeiten und die wesentlichen Einschränkungen. Das Erstellungsdatum wird nicht als neuer Datenstand ausgegeben. Die lokalen HTML-/Markdown-Exporte zeigen Spannen und Abrufzeitpunkte statt synthetischer Sterne-Ratings.

Das Dashboard enthält zusätzlich einen ausdrücklich bezeichneten **Modellanteil**: `Spannen-Näherungswert / (sichtbare Rezensionen + Näherungswert)`. Das ist keine gemessene Löschquote: Hinweiszeitraum und sichtbarer Rezensionsbestand haben unterschiedliche Zeitbezüge.

Das **1-Stern-Szenario** ist im Dashboard als Übersichtskachel, Tabellenansicht und sortierbare Spalte verfügbar: `(Google-Rating × sichtbare Rezensionen + Näherungswert × 1) / (sichtbare Rezensionen + Näherungswert)`. Es verwendet Spannenmittelpunkte und für „über 250“ den frei gewählten Rechenwert 300, nicht die unbekannte tatsächliche Anzahl oder Sternezahl. Es ist kein tatsächliches Rating und keine gesicherte Untergrenze. Ausgangsrating und Eingaben sind bereits gerundet; zwei Nachkommastellen sind keine Genauigkeitszusage. Die Berechnung erfolgt im Browser aus den vorhandenen Eingaben, ohne das frühere Modellfeld aus den Quelldaten zu übernehmen. Ohne beobachteten Hinweis oder gültige Eingaben wird kein Szenariowert angezeigt. Die Methodik erläutert Annahmen, unterschiedliche Zeitbezüge und ein Rechenbeispiel.

## Veröffentlichungsgrenzen und offene Prüfungen

`make site` veröffentlicht ausschließlich Dashboard, konfigurierte Informationsseiten, CNAME, robots.txt und Sitemap. Separate Diagramme, alte Ranglisten, Metadaten und CSV-Dateien werden nicht kopiert. Ein neu gebautes Artefakt enthält auch keine alten Dateien aus einem vorherigen Build. Die HTML-Daten des Dashboards bleiben technisch auslesbar; die versionierten Quelldaten im Repository sind davon unabhängig. Vorhandene Downloads, Suchmaschinen- und Social-Media-Caches werden dadurch nicht zurückgerufen. Eine Veröffentlichung muss das alte Hosting-Artefakt vollständig ersetzen.

Vor Veröffentlichung müssen weiterhin Hostingverträge/Empfänger, internationale Übermittlungen und Aufbewahrung, die rechtliche Tragfähigkeit der Zustellanschrift sowie Interessenabwägung, Informationspflichten und Anwendbarkeit des Medienprivilegs geprüft werden. Die korrigierten Texte bestätigen diese Voraussetzungen nicht. Konkrete Beanstandungen müssen tatsächlich bearbeitet und historische Beobachtungen bei Bedarf nachgeprüft werden.

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

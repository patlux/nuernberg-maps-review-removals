package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

const (
	width  = 1800
	height = 2500
	green  = "#2e7d32"
	red    = "#c9332c"
	orange = "#ef7d16"
	grid   = "#dde3e8"
	text   = "#202124"
	muted  = "#6f7377"
)

type args struct {
	Input           string
	OutDir          string
	PNG             bool
	Top             int
	MinCleanReviews int
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
	for i := range rows {
		mapsreview.ApplyPlaceOverrides(&rows[i])
	}
	if err := os.MkdirAll(args.OutDir, 0o755); err != nil {
		return err
	}
	if err := writeMostRemovedList(rows, args.OutDir); err != nil {
		return err
	}

	charts := []struct {
		Scope string
		Rows  []mapsreview.Place
	}{{Scope: "overall", Rows: rows}}
	for _, group := range mapsreview.GroupByPostcode(rows) {
		if group.Postcode != "unknown" {
			charts = append(charts, struct {
				Scope string
				Rows  []mapsreview.Place
			}{Scope: group.Postcode, Rows: group.Rows})
		}
	}

	for _, chart := range charts {
		svg := makeChart(chart.Rows, chart.Scope, args)
		base := "nuernberg_overall_summary"
		if chart.Scope != "overall" {
			base = "nuernberg_" + chart.Scope + "_summary"
		}
		svgFile := filepath.Join(args.OutDir, base+".svg")
		if err := os.WriteFile(svgFile, []byte(svg), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", svgFile)
		if args.PNG {
			pngFile := filepath.Join(args.OutDir, base+".png")
			if err := exportPNG(svgFile, pngFile); err != nil {
				fmt.Printf("skip png %s: %v\n", pngFile, err)
			} else {
				fmt.Printf("wrote %s\n", pngFile)
			}
		}
	}
	return nil
}

func parseArgs(argv []string) (args, error) {
	out := args{Input: mapsreview.ResultsJSON, OutDir: "output/charts", Top: 30, MinCleanReviews: 100}
	for i := 0; i < len(argv); i++ {
		key, value, consume := splitArg(argv, i)
		switch key {
		case "--input":
			out.Input = value
		case "--out-dir":
			out.OutDir = value
		case "--top":
			out.Top = atoi(value)
		case "--min-clean-reviews":
			out.MinCleanReviews = atoi(value)
		case "--png":
			out.PNG = true
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
  go run ./cmd/charts --png
  go run ./cmd/charts --input output/places.json --out-dir output/charts --top 50

Options:
  --input <path>              Scrape results JSON. Default: output/places.json.
  --out-dir <path>            Chart output directory. Default: output/charts.
  --top <n>                   Rows per bar chart. Default: 30.
  --min-clean-reviews <n>     Minimum reviews for clean ranking. Default: 100.
  --png                       Export PNGs with ImageMagick's magick command, when available.`)
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

func makeChart(rows []mapsreview.Place, scope string, args args) string {
	cleanRows := mapsreview.ValidRows(rows)
	removedRows := filter(cleanRows, func(row mapsreview.Place) bool { return row.HasDefamationNotice && row.DeletionRatioPct != nil })

	highestRatio := append([]mapsreview.Place(nil), removedRows...)
	sort.SliceStable(highestRatio, func(i, j int) bool {
		return mapsreview.FloatValue(highestRatio[i].DeletionRatioPct) > mapsreview.FloatValue(highestRatio[j].DeletionRatioPct)
	})
	highestRatio = take(highestRatio, args.Top)

	worstAdjusted := append([]mapsreview.Place(nil), removedRows...)
	sort.SliceStable(worstAdjusted, func(i, j int) bool {
		return mapsreview.FloatValue(worstAdjusted[i].RealRatingAdjusted) < mapsreview.FloatValue(worstAdjusted[j].RealRatingAdjusted)
	})
	worstAdjusted = take(worstAdjusted, args.Top)

	cleanRanking := filter(cleanRows, func(row mapsreview.Place) bool {
		return !row.HasDefamationNotice && mapsreview.IntValue(row.ReviewCount) >= args.MinCleanReviews
	})
	sort.SliceStable(cleanRanking, func(i, j int) bool {
		if mapsreview.FloatValue(cleanRanking[i].Rating) != mapsreview.FloatValue(cleanRanking[j].Rating) {
			return mapsreview.FloatValue(cleanRanking[i].Rating) > mapsreview.FloatValue(cleanRanking[j].Rating)
		}
		return mapsreview.IntValue(cleanRanking[i].ReviewCount) > mapsreview.IntValue(cleanRanking[j].ReviewCount)
	})
	cleanRanking = take(cleanRanking, args.Top)

	title := "Nürnberg — Gesamtstadt"
	subtitle := "Alle erfassten PLZ — gelöschte Rezensionen wegen „Diffamierung“"
	if scope != "overall" {
		title = "Postleitzahl " + scope + " — Nürnberg"
		subtitle = "Google-Maps-Orte: gelöschte Rezensionen wegen „Diffamierung“"
	}
	stats := fmt.Sprintf("%s Orte erfasst · %s mit sichtbarem Banner (%s%%) · %s in der „Über“-Stufe",
		mapsreview.FormatGermanInt(len(cleanRows)),
		mapsreview.FormatGermanInt(len(removedRows)),
		mapsreview.FormatGermanFloat((float64(len(removedRows))/math.Max(float64(len(cleanRows)), 1))*100, 1),
		mapsreview.FormatGermanInt(countRows(removedRows, func(row mapsreview.Place) bool { return row.RemovedMax == nil })),
	)

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height))
	b.WriteString(chartFrame(title, subtitle, stats))
	b.WriteString(drawBarChart(80, 230, 470, "1. Höchste Lösch-Quote", "Anteil gelöschter Rezensionen", highestRatio, red,
		func(row mapsreview.Place) float64 { return mapsreview.FloatValue(row.DeletionRatioPct) },
		maxFloat(10, values(highestRatio, func(row mapsreview.Place) float64 { return mapsreview.FloatValue(row.DeletionRatioPct) })),
		func(row mapsreview.Place) string {
			return mapsreview.FormatPtrFloat(row.DeletionRatioPct, 1) + "%  (" + mapsreview.RemovedRange(row) + " von " + mapsreview.FormatPtrInt(row.ReviewCount) + ")"
		},
		fmt.Sprintf("Lösch-Quote (%%) · Top %d", args.Top),
	))
	b.WriteString(drawBarChart(665, 230, 470, "2. Schlechtestes „echtes“ Rating", "Modell: gelöschte Rezensionen waren 1★", worstAdjusted, orange,
		func(row mapsreview.Place) float64 { return 5 - mapsreview.FloatValue(row.RealRatingAdjusted) },
		maxFloat(1, values(worstAdjusted, func(row mapsreview.Place) float64 { return 5 - mapsreview.FloatValue(row.RealRatingAdjusted) })),
		func(row mapsreview.Place) string {
			return mapsreview.FormatPtrFloat(row.RealRatingAdjusted, 2) + "★  (vorher " + mapsreview.FormatPtrFloat(row.Rating, 1) + "★)"
		},
		"Punkte unter 5★",
	))
	b.WriteString(drawBarChart(1250, 230, 470, "3. Beste „saubere“ Orte", fmt.Sprintf("kein sichtbarer Banner — ab %d Rezensionen", args.MinCleanReviews), cleanRanking, green,
		func(row mapsreview.Place) float64 { return mapsreview.FloatValue(row.Rating) },
		5,
		func(row mapsreview.Place) string {
			return mapsreview.FormatPtrFloat(row.Rating, 1) + "★  (" + mapsreview.FormatPtrInt(row.ReviewCount) + ")"
		},
		"Rating (★)",
	))
	b.WriteString(drawDistribution(cleanRows, 120, 1660, fmt.Sprintf("Verteilung der Lösch-Stufen über alle %s Orte", mapsreview.FormatGermanInt(len(cleanRows)))))
	b.WriteString(fmt.Sprintf(`<text x="40" y="2460" font-family="Arial, sans-serif" font-size="13" fill="#8a8f94">Quelle: Google Maps, öffentlich sichtbare Banner; Analyse-Snapshot %s</text>`, time.Now().Format("02.01.2006")))
	b.WriteString(`<text x="1760" y="2460" text-anchor="end" font-family="Arial, sans-serif" font-size="13" fill="#8a8f94">generated by Go cmd/charts</text>`)
	b.WriteString("</svg>\n")
	return b.String()
}

func chartFrame(title, subtitle, stats string) string {
	return fmt.Sprintf(`
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0%%" stop-color="#d9e8f2"/><stop offset="58%%" stop-color="#eef6fa"/><stop offset="100%%" stop-color="#d7dde2"/></linearGradient>
    <filter id="softShadow" x="-10%%" y="-10%%" width="120%%" height="120%%"><feDropShadow dx="0" dy="8" stdDeviation="10" flood-color="#6d7f8a" flood-opacity="0.16"/></filter>
  </defs>
  <rect width="100%%" height="100%%" fill="url(#bg)"/>
  <circle cx="1450" cy="310" r="500" fill="#ffffff" opacity="0.18"/>
  <circle cx="260" cy="2140" r="620" fill="#ffffff" opacity="0.20"/>
  <text x="900" y="70" text-anchor="middle" font-family="Arial, sans-serif" font-size="48" font-weight="800" fill="%s">%s</text>
  <text x="900" y="112" text-anchor="middle" font-family="Arial, sans-serif" font-size="22" fill="#4b5055">%s</text>
  <text x="900" y="160" text-anchor="middle" font-family="Arial, sans-serif" font-size="18" fill="%s">%s</text>`, text, esc(title), esc(subtitle), muted, esc(stats))
}

func drawBarChart(x, y, chartWidth int, titleText, subtitle string, rows []mapsreview.Place, color string, value func(mapsreview.Place) float64, maxValue float64, label func(mapsreview.Place) string, axisLabel string) string {
	plotX := x + 34
	plotY := y + 92
	plotW := chartWidth - 68
	rowH := 34.0
	if len(rows) > 0 {
		rowH = math.Max(24, math.Min(36, float64(1280-126)/float64(len(rows))))
	}
	actualH := rowH*math.Max(float64(len(rows)), 1) + 24
	max := math.Max(maxValue, 1)

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<g><text x="%d" y="%d" font-family="Arial, sans-serif" font-size="27" font-weight="800" fill="%s">%s</text>`, x, y, text, esc(titleText)))
	b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="Arial, sans-serif" font-size="16" fill="%s">%s</text>`, x, y+30, muted, esc(subtitle)))
	b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%.0f" fill="#ffffff" opacity="0.96" filter="url(#softShadow)"/>`, x, plotY, chartWidth, actualH))
	for i := 0; i <= 4; i++ {
		gx := float64(plotX) + float64(plotW*i)/4
		b.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%d" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`, gx, plotY, gx, float64(plotY)+actualH-20, grid))
	}
	for i, row := range rows {
		yy := float64(plotY) + 24 + float64(i)*rowH
		barW := math.Max(3, math.Min(float64(plotW), value(row)/max*float64(plotW)))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%.1f" text-anchor="end" font-family="Arial, sans-serif" font-size="13" fill="#83888d">%d.</text>`, x-8, yy+15, i+1))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%.1f" font-family="Arial, sans-serif" font-size="14" fill="%s">%s</text>`, plotX, yy-4, text, esc(trunc(row.Name, 45))))
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%.1f" width="%.1f" height="18" fill="%s"/>`, plotX, yy, barW, color))
		b.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" font-family="Arial, sans-serif" font-size="13" fill="%s">%s</text>`, float64(plotX)+barW+8, yy+15, text, esc(label(row))))
	}
	if axisLabel != "" {
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%.1f" text-anchor="middle" font-family="Arial, sans-serif" font-size="13" fill="%s">%s</text>`, plotX+plotW/2, float64(plotY)+actualH+18, muted, esc(axisLabel)))
	}
	b.WriteString(`</g>`)
	return b.String()
}

func drawDistribution(rows []mapsreview.Place, x, y int, titleText string) string {
	counts := map[string]int{}
	for _, bin := range mapsreview.BinOrder {
		counts[bin] = 0
	}
	for _, row := range rows {
		counts[mapsreview.BinFor(row)]++
	}
	maxCount := 1
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}
	colors := map[string]string{
		"Keine Löschung": green, "Eine gelöscht": "#8bc34a", "2–5 gelöscht": "#cddc39", "6–10 gelöscht": "#ffca28",
		"11–20 gelöscht": "#ff9f1c", "21–50 gelöscht": "#ff6d00", "51–100 gelöscht": "#e5391b", "101–200 gelöscht": "#c62828",
		"201–250 gelöscht": "#8e1919", "Über 250 gelöscht": "#5b0f0f",
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<g><text x="%d" y="%d" font-family="Arial, sans-serif" font-size="28" font-weight="800" fill="%s">%s</text>`, x, y, text, esc(titleText)))
	b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="1540" height="560" fill="#ffffff" opacity="0.96" filter="url(#softShadow)"/>`, x, y+55))
	for i, bin := range mapsreview.BinOrder {
		count := counts[bin]
		pct := float64(count) / math.Max(float64(len(rows)), 1) * 100
		yy := y + 105 + i*45
		barW := float64(count) / float64(maxCount) * 900
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="Arial, sans-serif" font-size="18" fill="%s">%s</text>`, x+30, yy+16, text, esc(bin)))
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="900" height="24" fill="#edf2f5"/>`, x+330, yy))
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%.1f" height="24" fill="%s"/>`, x+330, yy, barW, colors[bin]))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="Arial, sans-serif" font-size="18" fill="%s">%s (%s%%)</text>`, x+1260, yy+18, text, mapsreview.FormatGermanInt(count), mapsreview.FormatGermanFloat(pct, 1)))
	}
	b.WriteString(`</g>`)
	return b.String()
}

func writeMostRemovedList(rows []mapsreview.Place, outDir string) error {
	ranked := filter(mapsreview.ValidRows(rows), func(row mapsreview.Place) bool { return row.HasDefamationNotice && row.RemovedMin != nil })
	sort.SliceStable(ranked, func(i, j int) bool {
		if mapsreview.RemovedSortValue(ranked[i]) != mapsreview.RemovedSortValue(ranked[j]) {
			return mapsreview.RemovedSortValue(ranked[i]) > mapsreview.RemovedSortValue(ranked[j])
		}
		if mapsreview.IntValue(ranked[i].RemovedMin) != mapsreview.IntValue(ranked[j].RemovedMin) {
			return mapsreview.IntValue(ranked[i].RemovedMin) > mapsreview.IntValue(ranked[j].RemovedMin)
		}
		if mapsreview.IntValue(ranked[i].ReviewCount) != mapsreview.IntValue(ranked[j].ReviewCount) {
			return mapsreview.IntValue(ranked[i].ReviewCount) > mapsreview.IntValue(ranked[j].ReviewCount)
		}
		return ranked[i].Name < ranked[j].Name
	})
	if err := writeMostRemovedCSV(filepath.Join(outDir, "nuernberg_most_removed.csv"), ranked); err != nil {
		return err
	}
	if err := writeMostRemovedMD(filepath.Join(outDir, "nuernberg_most_removed.md"), ranked); err != nil {
		return err
	}
	if err := writeMostRemovedHTML(filepath.Join(outDir, "nuernberg_most_removed.html"), ranked, rows); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", filepath.Join(outDir, "nuernberg_most_removed.csv"))
	fmt.Printf("wrote %s\n", filepath.Join(outDir, "nuernberg_most_removed.md"))
	fmt.Printf("wrote %s\n", filepath.Join(outDir, "nuernberg_most_removed.html"))
	return nil
}

func writeMostRemovedCSV(file string, rows []mapsreview.Place) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	columns := []string{"rank", "name", "postcode", "address", "rating", "reviewCount", "removedRange", "removedMin", "removedMax", "removedEstimate", "deletionRatioPct", "realRatingAdjusted", "removedText", "url"}
	if err := w.Write(columns); err != nil {
		return err
	}
	for i, row := range rows {
		if err := w.Write([]string{
			strconv.Itoa(i + 1), row.Name, mapsreview.StringValue(row.Postcode), mapsreview.StringValue(row.Address), floatCSV(row.Rating), intCSV(row.ReviewCount), mapsreview.RemovedRange(row), intCSV(row.RemovedMin), intCSV(row.RemovedMax), strconv.FormatFloat(mapsreview.RemovedSortValue(row), 'f', -1, 64), floatCSV(row.DeletionRatioPct), floatCSV(row.RealRatingAdjusted), mapsreview.StringValue(row.RemovedText), row.URL,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeMostRemovedMD(file string, rows []mapsreview.Place) error {
	lines := []string{
		"# Nürnberg — Orte sortiert nach geschätzter Anzahl entfernter Bewertungen",
		"",
		"Quelle: Google Maps, öffentlich sichtbare Diffamierungs-Banner. Snapshot: " + time.Now().Format("02.01.2006") + ".",
		"",
		"Sortierung: geschätzter Mittelpunkt der Google-Bereiche absteigend. „Über 250“ wird als 300 geschätzt.",
		"",
		"Einträge mit Banner: " + mapsreview.FormatGermanInt(len(rows)),
		"",
		"| Rang | Name | PLZ | Rating | Rezensionen | Gelöscht | Schätzwert | Löschquote | Worst-Case-Rating |",
		"|---:|---|---:|---:|---:|---:|---:|---:|---:|",
	}
	for i, row := range rows {
		name := strings.ReplaceAll(row.Name, "|", "\\|")
		lines = append(lines, fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %s | %s%% | %s |",
			i+1, name, mapsreview.StringValue(row.Postcode), mapsreview.FormatPtrFloat(row.Rating, 1), mapsreview.FormatPtrInt(row.ReviewCount), mapsreview.RemovedRange(row), mapsreview.FormatGermanFloat(mapsreview.RemovedSortValue(row), 1), mapsreview.FormatPtrFloat(row.DeletionRatioPct, 1), mapsreview.FormatPtrFloat(row.RealRatingAdjusted, 2)))
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func writeMostRemovedHTML(file string, ranked []mapsreview.Place, allRows []mapsreview.Place) error {
	var body strings.Builder
	for i, row := range ranked {
		body.WriteString(fmt.Sprintf(`<tr><td class="num">%d</td><td><a href="%s" target="_blank" rel="noopener noreferrer">%s</a><small>%s</small></td><td>%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s%%</td><td class="num">%s</td></tr>`,
			i+1, escAttr(row.URL), esc(row.Name), esc(mapsreview.StringValue(row.Address)), esc(mapsreview.StringValue(row.Postcode)), mapsreview.FormatPtrFloat(row.Rating, 1), mapsreview.FormatPtrInt(row.ReviewCount), esc(mapsreview.RemovedRange(row)), mapsreview.FormatGermanFloat(mapsreview.RemovedSortValue(row), 1), mapsreview.FormatPtrFloat(row.DeletionRatioPct, 1), mapsreview.FormatPtrFloat(row.RealRatingAdjusted, 2)))
	}
	htmlText := fmt.Sprintf(`<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Nürnberg — meist entfernte Google-Maps-Bewertungen</title><style>
:root{--red:#c9332c;--text:#202124;--muted:#687078;--line:#dce5eb;--bg:#eef6fa}*{box-sizing:border-box}body{margin:0;font-family:Georgia,serif;background:linear-gradient(180deg,#d9e8f2,#f7fbfd 48%%,#e1e7ec);color:var(--text)}main{width:min(1450px,calc(100vw - 40px));margin:42px auto 80px}h1{font-size:clamp(34px,5vw,64px);line-height:1;margin:0 0 12px;letter-spacing:-.04em}.lead{max-width:900px;color:var(--muted);font:18px/1.5 system-ui,sans-serif}.stats{display:flex;gap:12px;flex-wrap:wrap;margin:24px 0}.stat{background:#fff;border:1px solid var(--line);border-left:8px solid var(--red);padding:14px 18px}.stat strong{display:block;font:700 30px/1 system-ui,sans-serif}.table-wrap{overflow:auto;background:#fff;border:1px solid var(--line);box-shadow:0 14px 42px rgba(60,80,95,.16)}table{width:100%%;border-collapse:collapse;min-width:1100px}th,td{padding:12px 14px;border-bottom:1px solid #edf2f5;text-align:left;font:14px system-ui,sans-serif}th{position:sticky;top:0;background:#f8fbfd;font-weight:800}.num{text-align:right;font-variant-numeric:tabular-nums}a{color:var(--red);font-weight:800;text-decoration:none}a:hover{text-decoration:underline}small{display:block;color:var(--muted);margin-top:4px}</style></head><body><main><h1>Nürnberg — meist entfernte Google-Maps-Bewertungen</h1><p class="lead">Orte mit sichtbarem Google-Maps-Hinweis auf entfernte Bewertungen wegen Beschwerden wegen Diffamierung. Namen sind direkt zur jeweiligen Google-Maps-Seite verlinkt.</p><div class="stats"><div class="stat"><strong>%s</strong><span>Einträge mit Banner</span></div><div class="stat"><strong>%s</strong><span>erfasste Orte</span></div><div class="stat"><strong>%s%%</strong><span>mit sichtbarem Banner</span></div></div><section class="table-wrap"><table><thead><tr><th class="num">Rang</th><th>Name / Google Maps</th><th>PLZ</th><th class="num">Rating</th><th class="num">Rezensionen</th><th class="num">Gelöscht</th><th class="num">Schätzwert</th><th class="num">Löschquote</th><th class="num">Worst-Case</th></tr></thead><tbody>%s</tbody></table></section></main></body></html>`,
		mapsreview.FormatGermanInt(len(ranked)), mapsreview.FormatGermanInt(len(mapsreview.ValidRows(allRows))), mapsreview.FormatGermanFloat(float64(len(ranked))/math.Max(float64(len(mapsreview.ValidRows(allRows))), 1)*100, 1), body.String())
	return os.WriteFile(file, []byte(htmlText), 0o644)
}

func exportPNG(svgFile, pngFile string) error {
	if _, err := exec.LookPath("magick"); err != nil {
		return fmt.Errorf("ImageMagick magick not found")
	}
	cmd := exec.Command("magick", svgFile, pngFile)
	return cmd.Run()
}

func filter(rows []mapsreview.Place, keep func(mapsreview.Place) bool) []mapsreview.Place {
	out := make([]mapsreview.Place, 0, len(rows))
	for _, row := range rows {
		if keep(row) {
			out = append(out, row)
		}
	}
	return out
}

func take(rows []mapsreview.Place, n int) []mapsreview.Place {
	if n > 0 && len(rows) > n {
		return rows[:n]
	}
	return rows
}

func countRows(rows []mapsreview.Place, keep func(mapsreview.Place) bool) int {
	count := 0
	for _, row := range rows {
		if keep(row) {
			count++
		}
	}
	return count
}

func values(rows []mapsreview.Place, value func(mapsreview.Place) float64) []float64 {
	out := make([]float64, 0, len(rows))
	for _, row := range rows {
		out = append(out, value(row))
	}
	return out
}

func maxFloat(fallback float64, values []float64) float64 {
	maxValue := fallback
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func trunc(value string, maxLen int) string {
	if len([]rune(value)) <= maxLen {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxLen-1]) + "…"
}

func esc(value string) string     { return html.EscapeString(value) }
func escAttr(value string) string { return html.EscapeString(value) }

func intCSV(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func floatCSV(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

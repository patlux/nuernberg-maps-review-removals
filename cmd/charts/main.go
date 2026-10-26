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
	City            string
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
	if err := writeMostRemovedList(rows, args); err != nil {
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

	filePrefix := chartFilePrefix(args.City)
	for _, chart := range charts {
		svg := makeChart(chart.Rows, chart.Scope, args)
		base := filePrefix + "_overall_summary"
		if chart.Scope != "overall" {
			base = filePrefix + "_" + chart.Scope + "_summary"
		}
		svgFile := filepath.Join(args.OutDir, base+".svg")
		if err := os.WriteFile(svgFile, []byte(svg), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", svgFile)
		if args.PNG {
			pngFile := filepath.Join(args.OutDir, base+".png")
			if err := exportPNG(svgFile, pngFile); err != nil {
				return fmt.Errorf("export png %s: %w", pngFile, err)
			} else {
				fmt.Printf("wrote %s\n", pngFile)
			}
		}
	}
	return nil
}

func parseArgs(argv []string) (args, error) {
	out := args{
		City:            mapsreview.DefaultCity,
		Input:           mapsreview.ResultsJSON,
		OutDir:          "output/charts",
		Top:             30,
		MinCleanReviews: 100}
	for i := 0; i < len(argv); i++ {
		key, value, consume := mapsreview.SplitArg(argv, i)
		switch key {
		case "--city":
			out.City = value
		case "--input":
			out.Input = value
		case "--out-dir":
			out.OutDir = value
		case "--top":
			out.Top = mapsreview.Atoi(value)
		case "--min-clean-reviews":
			out.MinCleanReviews = mapsreview.Atoi(value)
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
	out.City = strings.TrimSpace(out.City)
	if out.City == "" {
		return out, fmt.Errorf("--city must not be empty")
	}
	return out, nil
}

func printHelp() {
	fmt.Printf(`Usage:
  go run ./cmd/charts --png
  go run ./cmd/charts --input output/places.json --out-dir output/charts --top 50

Options:
  --city <name>               Displayed city name. Default: %s.
  --input <path>              Scrape results JSON. Default: output/places.json.
  --out-dir <path>            Chart output directory. Default: output/charts.
  --top <n>                   Rows per bar chart. Default: 30.
  --min-clean-reviews <n>     Minimum reviews for clean ranking. Default: 100.
  --png                       Export PNGs with ImageMagick's magick command, when available.\n`, mapsreview.DefaultCity)
}

func makeChart(rows []mapsreview.Place, scope string, a args) string {
	valid := mapsreview.ValidRows(rows)
	observed := filter(valid, func(row mapsreview.Place) bool { return row.HasDefamationNotice })
	highest := append([]mapsreview.Place(nil), observed...)
	sort.SliceStable(highest, func(i, j int) bool {
		if mapsreview.RemovedSortValue(highest[i]) != mapsreview.RemovedSortValue(highest[j]) {
			return mapsreview.RemovedSortValue(highest[i]) > mapsreview.RemovedSortValue(highest[j])
		}
		return highest[i].Name < highest[j].Name
	})
	highest = take(highest, a.Top)
	categories := map[string]int{}
	for _, row := range observed {
		category := mapsreview.StringValue(row.Category)
		if category == "" {
			category = "Ohne Kategorie"
		}
		categories[category]++
	}
	groups := []mapsreview.Place{}
	for category, count := range categories {
		groups = append(groups, mapsreview.Place{Name: category, ReviewCount: mapsreview.IntPtr(count)})
	}
	sort.Slice(groups, func(i, j int) bool {
		if *groups[i].ReviewCount != *groups[j].ReviewCount {
			return *groups[i].ReviewCount > *groups[j].ReviewCount
		}
		return groups[i].Name < groups[j].Name
	})
	groups = take(groups, a.Top)
	clean := filter(valid, func(row mapsreview.Place) bool {
		return !row.HasDefamationNotice && mapsreview.IntValue(row.ReviewCount) >= a.MinCleanReviews
	})
	sort.SliceStable(clean, func(i, j int) bool {
		if *clean[i].Rating != *clean[j].Rating {
			return *clean[i].Rating > *clean[j].Rating
		}
		if *clean[i].ReviewCount != *clean[j].ReviewCount {
			return *clean[i].ReviewCount > *clean[j].ReviewCount
		}
		return clean[i].Name < clean[j].Name
	})
	clean = take(clean, a.Top)
	title := a.City + " — erfasste Orte"
	if scope != "overall" {
		title = fmt.Sprintf("Postleitzahl %s — %s", scope, a.City)
	}
	stats := fmt.Sprintf("%s Orte mit Rating und Rezensionszahl · %s mit beobachtetem Google-Hinweis (%s%%)", mapsreview.FormatGermanInt(len(valid)), mapsreview.FormatGermanInt(len(observed)), mapsreview.FormatGermanFloat(float64(len(observed))/math.Max(float64(len(valid)), 1)*100, 1))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)
	b.WriteString(chartFrame(title, "Historische Momentaufnahme · Google-Hinweise auf Entfernungen wegen Diffamierungsbeschwerden", stats))
	b.WriteString(drawBarChart(80, 230, 470, "1. Beobachtete Spannen", "Gleiche Spanne: alphabetisch, nicht nach Qualität", highest, "#5b7b8e",
		mapsreview.RemovedSortValue, maxFloat(1, values(highest, mapsreview.RemovedSortValue)), mapsreview.RemovedRange, "Balken: Spannenmittelpunkt; über 250 = 300"))
	b.WriteString(drawBarChart(665, 230, 470, "2. Hinweise nach Kategorie", "Anzahl erfasster Orte mit Hinweis je Kategorie", groups, "#5b7b8e",
		func(row mapsreview.Place) float64 { return float64(*row.ReviewCount) }, maxFloat(1, values(groups, func(row mapsreview.Place) float64 { return float64(*row.ReviewCount) })),
		func(row mapsreview.Place) string { return mapsreview.FormatPtrInt(row.ReviewCount) + " Orte" }, "Anzahl Orte, nicht Anzahl Entfernungen"))
	b.WriteString(drawBarChart(1250, 230, 470, "3. Ohne beobachteten Hinweis", fmt.Sprintf("Nach Google-Rating; ab %d Rezensionen", a.MinCleanReviews), clean, "#8a8e94",
		func(row mapsreview.Place) float64 { return *row.Rating }, 5,
		func(row mapsreview.Place) string {
			return mapsreview.FormatPtrFloat(row.Rating, 1) + "★ · " + mapsreview.FormatPtrInt(row.ReviewCount) + " Rez."
		}, "Kein Hinweis bedeutet nicht: keine Entfernungen"))
	b.WriteString(drawDistribution(valid, 120, 1660, "Beobachtete Google-Spannen in der erfassten Stichprobe"))
	for i, line := range []string{
		"Historische Momentaufnahme · Datenabrufe: " + chartDataPeriod(rows) + ". Kein aktueller Status und keine vollständige Stadterhebung.",
		"Die Google-Hinweise belegen weder rechtswidriges Verhalten noch missbräuchliche Beschwerden oder unterdrückte berechtigte Kritik.",
		"Spannen beziehen sich auf den damaligen Google-Hinweis; die genaue Zahl und Sterne entfernter Rezensionen sind unbekannt.",
		"Quelle: Google Maps. Kein Hinweis beim Abruf erlaubt keinen Rückschluss auf die Anzahl entfernter Rezensionen.",
	} {
		fmt.Fprintf(&b, `<text x="120" y="%d" font-family="Arial, sans-serif" font-size="18" fill="#4b5055">%s</text>`, 2320+i*32, esc(line))
	}
	b.WriteString(`<text x="1680" y="2480" text-anchor="end" font-family="Arial, sans-serif" font-size="15" fill="#687078">Patrick Wozniak · patwoz.dev</text></svg>`)
	return b.String()
}

func chartDataPeriod(rows []mapsreview.Place) string {
	var first, last time.Time
	for _, row := range rows {
		t, err := time.Parse(time.RFC3339Nano, row.ReadAt)
		if err != nil {
			continue
		}
		if first.IsZero() || t.Before(first) {
			first = t
		}
		if last.IsZero() || t.After(last) {
			last = t
		}
	}
	if first.IsZero() {
		return "unbekannt"
	}
	if first.Format("02.01.2006") == last.Format("02.01.2006") {
		return first.Format("02.01.2006")
	}
	return first.Format("02.01.2006") + "–" + last.Format("02.01.2006")
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
		barW := math.Max(3, math.Min(float64(plotW-160), value(row)/max*float64(plotW-160)))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%.1f" text-anchor="end" font-family="Arial, sans-serif" font-size="13" fill="#83888d">%d.</text>`, x-8, yy+15, i+1))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%.1f" font-family="Arial, sans-serif" font-size="14" fill="%s">%s</text>`, plotX, yy-4, text, esc(trunc(row.Name, 45))))
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%.1f" width="%.1f" height="18" fill="%s"/>`, plotX, yy, barW, color))
		b.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" text-anchor="end" font-family="Arial, sans-serif" font-size="13" fill="%s">%s</text>`, float64(plotX+plotW), yy+15, text, esc(label(row))))
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
		"Kein Google-Hinweis beim Abruf": "#8a8e94", "Eine gelöscht": "#8bc34a", "2–5 gelöscht": "#cddc39", "6–10 gelöscht": "#ffca28",
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
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="Arial, sans-serif" font-size="16" fill="%s">%s</text>`, x+30, yy+16, text, esc(bin)))
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="900" height="24" fill="#edf2f5"/>`, x+330, yy))
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%.1f" height="24" fill="%s"/>`, x+330, yy, barW, colors[bin]))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="Arial, sans-serif" font-size="18" fill="%s">%s (%s%%)</text>`, x+1260, yy+18, text, mapsreview.FormatGermanInt(count), mapsreview.FormatGermanFloat(pct, 1)))
	}
	b.WriteString(`</g>`)
	return b.String()
}

func writeMostRemovedList(rows []mapsreview.Place, args args) error {
	ranked := filter(mapsreview.ValidRows(rows), func(row mapsreview.Place) bool { return row.HasDefamationNotice && row.RemovedMin != nil })
	sort.SliceStable(ranked, func(i, j int) bool {
		if mapsreview.RemovedSortValue(ranked[i]) != mapsreview.RemovedSortValue(ranked[j]) {
			return mapsreview.RemovedSortValue(ranked[i]) > mapsreview.RemovedSortValue(ranked[j])
		}
		if mapsreview.IntValue(ranked[i].RemovedMin) != mapsreview.IntValue(ranked[j].RemovedMin) {
			return mapsreview.IntValue(ranked[i].RemovedMin) > mapsreview.IntValue(ranked[j].RemovedMin)
		}
		return ranked[i].Name < ranked[j].Name
	})
	filePrefix := chartFilePrefix(args.City)
	csvFile := filepath.Join(args.OutDir, filePrefix+"_most_removed.csv")
	mdFile := filepath.Join(args.OutDir, filePrefix+"_most_removed.md")
	htmlFile := filepath.Join(args.OutDir, filePrefix+"_most_removed.html")
	if err := writeMostRemovedCSV(csvFile, ranked); err != nil {
		return err
	}
	if err := writeMostRemovedMD(mdFile, ranked, args); err != nil {
		return err
	}
	if err := writeMostRemovedHTML(htmlFile, ranked, rows, args); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", csvFile)
	fmt.Printf("wrote %s\n", mdFile)
	fmt.Printf("wrote %s\n", htmlFile)
	return nil
}

func writeMostRemovedCSV(file string, rows []mapsreview.Place) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	columns := []string{"rank", "name", "postcode", "address", "rating", "reviewCount", "removedRange", "removedMin", "removedMax", "removedEstimate", "deletionRatioPct", "readAt", "url"}
	if err := w.Write(columns); err != nil {
		return err
	}
	for i, row := range rows {
		if err := w.Write([]string{
			strconv.Itoa(i + 1), row.Name, mapsreview.StringValue(row.Postcode), mapsreview.StringValue(row.Address), floatCSV(row.Rating), intCSV(row.ReviewCount), mapsreview.RemovedRange(row), intCSV(row.RemovedMin), intCSV(row.RemovedMax), strconv.FormatFloat(mapsreview.RemovedSortValue(row), 'f', -1, 64), floatCSV(row.DeletionRatioPct), row.ReadAt, row.URL,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeMostRemovedMD(file string, rows []mapsreview.Place, a args) error {
	lines := []string{
		fmt.Sprintf("# %s — beobachtete Google-Hinweisspannen", esc(a.City)), "",
		"Historische Momentaufnahme. Datenabrufe: " + chartDataPeriod(rows) + ". Quelle: Google Maps.", "",
		"Die Spannen sind keine exakten Zahlen. Gleiche Spanne: alphabetisch. Keine Aussage über Betriebsqualität, Rechtswidrigkeit, Missbrauch oder unterdrückte berechtigte Kritik.", "",
		"| Position | Name | PLZ | Google-Rating | Rezensionen | Beobachtete Spanne | Datenabruf |",
		"|---:|---|---|---:|---:|---|---|",
	}
	for i, row := range rows {
		name := strings.ReplaceAll(esc(row.Name), "|", "&#124;")
		lines = append(lines, fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %s |", i+1, name, esc(mapsreview.StringValue(row.Postcode)), mapsreview.FormatPtrFloat(row.Rating, 1), mapsreview.FormatPtrInt(row.ReviewCount), esc(mapsreview.RemovedRange(row)), esc(row.ReadAt)))
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func writeMostRemovedHTML(file string, ranked []mapsreview.Place, allRows []mapsreview.Place, a args) error {
	var body strings.Builder
	for i, row := range ranked {
		fmt.Fprintf(&body, `<tr><td>%d</td><td><a href="%s" target="_blank" rel="noopener noreferrer">%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`, i+1, esc(row.URL), esc(row.Name), esc(mapsreview.StringValue(row.Postcode)), mapsreview.FormatPtrFloat(row.Rating, 1), mapsreview.FormatPtrInt(row.ReviewCount), esc(mapsreview.RemovedRange(row)), esc(row.ReadAt))
	}
	page := fmt.Sprintf(`<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="robots" content="noindex"><title>%s — historische Google-Hinweisspannen</title><style>body{font:16px/1.6 system-ui,sans-serif;margin:32px;color:#202124}main{max-width:1400px;margin:auto}.table{overflow:auto}table{border-collapse:collapse;width:100%%}th,td{padding:12px;text-align:left;border-bottom:1px solid #ddd}a{color:#305b72}</style></head><body><main><h1>%s — historische Google-Hinweisspannen</h1><p>Lokaler Recherche-Export; nicht Bestandteil der Website-Veröffentlichung.</p><p><strong>Historische Momentaufnahme. Datenabrufe: %s.</strong> Quelle: Google Maps. Kein aktueller Status.</p><p>Beobachtete Spannen, innerhalb derselben Spanne alphabetisch. Keine Aussage über Betriebsqualität, rechtswidriges Verhalten, missbräuchliche Beschwerden oder unterdrückte berechtigte Kritik.</p><div class="table"><table><thead><tr><th>Position</th><th>Name / Google Maps</th><th>PLZ</th><th>Google-Rating</th><th>Rezensionen</th><th>Beobachtete Spanne</th><th>Datenabruf</th></tr></thead><tbody>%s</tbody></table></div></main></body></html>`, esc(a.City), esc(a.City), esc(chartDataPeriod(allRows)), body.String())
	return os.WriteFile(file, []byte(page), 0o644)
}

func chartFilePrefix(city string) string {
	if mapsreview.IsDefaultCity(city) {
		return "nuernberg"
	}
	city = strings.ToLower(strings.TrimSpace(city))
	city = strings.NewReplacer(
		"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
	).Replace(city)
	var b strings.Builder
	lastDash := false
	for _, r := range city {
		isASCIIAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isASCIIAlnum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	prefix := strings.Trim(b.String(), "-")
	if prefix == "" {
		return "city"
	}
	return prefix
}

func exportPNG(svgFile, pngFile string) error {
	if _, err := exec.LookPath("magick"); err == nil {
		return exec.Command("magick", svgFile, pngFile).Run()
	}
	if _, err := exec.LookPath("convert"); err == nil {
		return exec.Command("convert", svgFile, pngFile).Run()
	}
	return fmt.Errorf("ImageMagick magick/convert not found")
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

func esc(value string) string { return html.EscapeString(value) }

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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func TestChartsDescribeObservedDataWithoutSyntheticRankings(t *testing.T) {
	rows := []mapsreview.Place{
		{Name: "Example A", Status: "success", Rating: mapsreview.FloatPtr(4.5), ReviewCount: mapsreview.IntPtr(100), HasDefamationNotice: true, RemovedMin: mapsreview.IntPtr(21), RemovedMax: mapsreview.IntPtr(50), ReadAt: "2026-05-12T12:00:00Z"},
		{Name: "Example B", Status: "success", Rating: mapsreview.FloatPtr(4.6), ReviewCount: mapsreview.IntPtr(120), ReadAt: "2026-05-13T12:00:00Z"},
	}
	a := args{City: "Nürnberg", Top: 5, MinCleanReviews: 100}
	svg := makeChart(rows, "overall", a)
	for _, forbidden := range []string{"Schlechtestes", "echtes", "Beste Orte", "Keine Löschung", "Lösch-Quote", "Worst-Case"} {
		if strings.Contains(svg, forbidden) {
			t.Errorf("misleading chart label %q", forbidden)
		}
	}
	for _, want := range []string{"Historische Momentaufnahme", "12.05.2026–13.05.2026", "rechtswidriges Verhalten", "Kein Google-Hinweis beim Abruf", "Hinweise nach Kategorie"} {
		if !strings.Contains(svg, want) {
			t.Errorf("missing standalone context %q", want)
		}
	}
	dir := t.TempDir()
	if err := writeMostRemovedHTML(filepath.Join(dir, "list.html"), rows, rows, a); err != nil {
		t.Fatal(err)
	}
	if err := writeMostRemovedMD(filepath.Join(dir, "list.md"), rows, a); err != nil {
		t.Fatal(err)
	}
	if err := writeMostRemovedCSV(filepath.Join(dir, "list.csv"), rows); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"list.html", "list.md", "list.csv"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"Worst-Case", "realRatingAdjusted", "Löschquote", "removedText"} {
			if strings.Contains(string(data), forbidden) {
				t.Errorf("%s includes %q", name, forbidden)
			}
		}
		if !strings.Contains(string(data), "2026-05-12T12:00:00Z") {
			t.Errorf("%s missing source date", name)
		}
	}
}

func TestChartPeriodDoesNotUseBuildDate(t *testing.T) {
	if got := chartDataPeriod(nil); got != "unbekannt" {
		t.Fatalf("unknown data period = %s", got)
	}
}

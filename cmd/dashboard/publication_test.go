package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func TestSiteDoesNotPublishLegacyExports(t *testing.T) {
	dir := t.TempDir()
	a := args{City: "Nürnberg", Output: filepath.Join(dir, "charts", "dashboard.html"), SiteOutput: filepath.Join(dir, "public"), SiteURL: "https://example.test"}
	if err := os.MkdirAll(filepath.Dir(a.Output), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dashboard.html", "nuernberg_most_removed.html", "nuernberg_most_removed.csv", "nuernberg_overall_summary.png", "old.svg"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(a.Output), name), []byte("legacy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"data/places.csv", "charts/old.svg", "korrektur.html"} {
		file := filepath.Join(a.SiteOutput, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := buildSite(a); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"data/places.csv", "data/metadata.json", "charts/nuernberg_most_removed.html", "charts/nuernberg_most_removed.csv", "charts/nuernberg_overall_summary.png", "charts/old.svg", "korrektur.html"} {
		if _, err := os.Stat(filepath.Join(a.SiteOutput, name)); !os.IsNotExist(err) {
			t.Errorf("unintended publication: %s", name)
		}
	}
	sitemap, err := os.ReadFile(filepath.Join(a.SiteOutput, "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sitemap), "most_removed") {
		t.Fatal("legacy ranking in sitemap")
	}
}

func TestPublicDashboardOmitsUnsupportedRatingClaims(t *testing.T) {
	page := makeHTML(args{City: "Nürnberg"}, nil)
	for _, forbidden := range []string{"Worst-Case", "Mathematische Untergrenze", "realRatingAdjusted", "barsWorst", "Schlechtestes", "overall_summary.png"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("public dashboard contains %q", forbidden)
		}
	}
	if !strings.Contains(page, "Historische Momentaufnahme") {
		t.Error("missing archive notice")
	}
}

func TestUnknownSnapshotDoesNotInventCurrentDate(t *testing.T) {
	if !snapshotTime(nil).IsZero() {
		t.Fatal("unknown snapshot becomes today's date")
	}
}

func TestUnverifiedExternalServicesStayOff(t *testing.T) {
	a := args{City: "Nürnberg", AnalyticsSrc: "https://analytics.example.test/script.js", CartoAPIKey: "test-publishable-key"}
	page := makeHTML(a, nil)
	if strings.Contains(page, `src="https://analytics.example.test/script.js"`) {
		t.Error("unverified analytics enabled")
	}
	if strings.Contains(page, "https://unpkg.com/") {
		t.Error("Leaflet still loads from a third-party CDN")
	}
}

func TestMissingNoticeNeverMeansNoDeletion(t *testing.T) {
	if got := mapsreview.BinFor(mapsreview.Place{}); got != "Kein Google-Hinweis beim Abruf" {
		t.Fatalf("misleading bin: %q", got)
	}
}

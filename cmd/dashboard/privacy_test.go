package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalServicesRequireExplicitEnableAndDisclosures(t *testing.T) {
	for _, service := range []string{"carto", "analytics"} {
		t.Run(service, func(t *testing.T) {
			dir := t.TempDir()
			config := filepath.Join(dir, "config.toml")
			base := "[legal]\nenabled = true\nname = \"Example\"\nemail = \"example@example.test\"\naddress_lines = [\"Example Street 1\"]\n[" + service + "]\nenabled = true\n"
			if err := os.WriteFile(config, []byte(base), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := parseArgs([]string{"--config", config}); err == nil || !strings.Contains(err.Error(), "privacy_text") {
				t.Fatalf("missing disclosure accepted: %v", err)
			}
			complete := base + "privacy_text = \"Verified test provider and transfer disclosure\"\nsrc = \"https://analytics.example.test/script.js\"\n"
			if service == "carto" {
				complete = base + "privacy_text = \"Verified test provider and transfer disclosure\"\n"
			}
			if err := os.WriteFile(config, []byte(complete), 0o600); err != nil {
				t.Fatal(err)
			}
			a, err := parseArgs([]string{"--config", config})
			if err != nil {
				t.Fatal(err)
			}
			if service == "carto" {
				if !cartoTilesEnabled(a) {
					t.Fatal("configured tiles disabled")
				}
				for _, want := range []string{"Kartenausschnitt", "Verschieben", "Karte deaktivieren", "Verified test provider"} {
					if !strings.Contains(mapConsentHTML(a), want) {
						t.Errorf("missing map disclosure %q", want)
					}
				}
			} else {
				snippet := plausibleAnalyticsSnippet(a)
				if strings.Contains(snippet, "<script src=") || !strings.Contains(snippet, "if (consent())") {
					t.Fatal("analytics is not consent gated")
				}
				if !strings.Contains(plausiblePrivacyNotice(a), "analyticsConsent") {
					t.Fatal("missing withdrawal control")
				}
			}
		})
	}
}

func TestUnknownDateIsNotPublishedAsYearOne(t *testing.T) {
	page := makeHTML(args{City: "Nürnberg"}, nil)
	for _, forbidden := range []string{"0001-01-01", "og:updated_time", "dateModified"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("invented publication date: %q", forbidden)
		}
	}
}

func TestDatasetCoverageAndEstimateUseSourceValues(t *testing.T) {
	rows := []clientRow{{Name: "A", HasBanner: true, RemovedEstimate: 35.5, ReadAt: "2026-05-12T12:00:00Z"}, {Name: "B", HasBanner: true, RemovedEstimate: 35.5, ReadAt: "2026-05-13T12:00:00Z"}}
	stats := makeSEOStats(rows, "13.05.2026")
	if stats.RemovedEstimate != 71 {
		t.Fatalf("rounding changed total: %d", stats.RemovedEstimate)
	}
	var value struct {
		Graph []map[string]interface{} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(structuredDataJSON(stats, snapshotTime(rows), "Nürnberg", dashboardMeta(args{}, "Nürnberg"))), &value); err != nil {
		t.Fatal(err)
	}
	if got := value.Graph[2]["temporalCoverage"]; got != "2026-05-12/2026-05-13" {
		t.Fatalf("coverage = %v", got)
	}
	if got := snapshotTime(rows).Format("2006-01-02"); got != "2026-05-13" {
		t.Fatalf("snapshot = %s", got)
	}
}

func TestCompactRowsDoNotPublishSyntheticStars(t *testing.T) {
	var payload compactClientData
	if err := json.Unmarshal([]byte(compactClientDataJSON([]clientRow{{Name: "Example", HasBanner: true, RemovedRange: "21–50"}}, "Nürnberg")), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows[0]) != 15 {
		t.Fatalf("unexpected additional client fields: %d", len(payload.Rows[0]))
	}
}

func TestSiteRefusesSourcesAndProjectDirectories(t *testing.T) {
	for _, projectFile := range []string{".git", "go.mod", "config.toml"} {
		dir := t.TempDir()
		src := filepath.Join(dir, "dashboard.html")
		out := filepath.Join(dir, "public")
		if err := os.WriteFile(src, []byte("page"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(out, 0o755); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(out, projectFile)
		if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := buildSite(args{Output: src, SiteOutput: out}); err == nil {
			t.Fatal("accepted project output")
		}
		if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
			t.Fatal("damaged protected directory")
		}
	}
}

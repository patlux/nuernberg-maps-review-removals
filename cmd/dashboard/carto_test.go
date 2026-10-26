package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func dashboardArgsWithCartoKey(t *testing.T, key string) args {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	config := "[carto]\napi_key = " + strconv.Quote(key) + "\n[places_api]\napi_key = \"test-google-key-must-stay-private\"\n"
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := parseArgs([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func clientDashboardConfig(t *testing.T, page string) map[string]string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<script id="dashboardConfig" type="application/json">`)
	if !ok {
		t.Fatal("dashboard configuration element missing")
	}
	value, _, ok := strings.Cut(rest, "</script>")
	if !ok {
		t.Fatal("dashboard configuration element not closed")
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestCartoKeyFlowsFromConfigToDashboard(t *testing.T) {
	const key = "test-key-not-a-credential"
	a := dashboardArgsWithCartoKey(t, "  "+key+"  ")
	page := makeHTML(a, nil)
	if clientDashboardConfig(t, page)["cartoApiKey"] != key {
		t.Fatal("configured CARTO key was not passed to the client")
	}
	if strings.Contains(page, "test-google-key-must-stay-private") {
		t.Fatal("Google Places API key leaked into the dashboard")
	}
	for _, want := range []string{
		"encodeURIComponent(CONFIG.cartoApiKey)",
		"?key=",
		"currentTheme() === 'dark' ? 'dark_all' : 'light_all'",
		"tileLayer.setUrl(tileURL())",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("dashboard missing keyed tile URL fragment %q", want)
		}
	}
}

func TestCartoKeyIsSafelyEmbeddedAsJSON(t *testing.T) {
	const key = `test-</script><script>alert("x")</script>&?+`
	page := makeHTML(dashboardArgsWithCartoKey(t, key), nil)
	if strings.Contains(page, key) {
		t.Fatal("CARTO key was embedded as unescaped HTML")
	}
	if clientDashboardConfig(t, page)["cartoApiKey"] != key {
		t.Fatal("CARTO key did not survive JSON encoding")
	}
}

func TestCartoMalformedConfigDoesNotRevealKey(t *testing.T) {
	const sensitiveValue = "test-private-value"
	_, err := parseDashboardTOML("[carto]\napi_key = \"" + sensitiveValue + "\\q\"\n")
	if err == nil {
		t.Fatal("malformed CARTO configuration was accepted")
	}
	if strings.Contains(err.Error(), sensitiveValue) {
		t.Fatal("configuration error revealed the key")
	}
}

func TestRunRejectsMissingCartoKeyBeforeWritingArtifacts(t *testing.T) {
	for _, site := range []bool{false, true} {
		for _, key := range []string{"", " \t "} {
			a := defaultArgs()
			if key != "" {
				a = dashboardArgsWithCartoKey(t, key)
			}
			dir := t.TempDir()
			a.Input = filepath.Join(dir, "missing-input.json")
			a.Output = filepath.Join(dir, "dashboard.html")
			a.SiteOutput = filepath.Join(dir, "public")
			a.BuildSite = site
			if err := run(a); err == nil || !strings.Contains(err.Error(), "carto.api_key is required") {
				t.Fatal("build did not reject a missing CARTO key before reading input")
			}
			for _, path := range []string{a.Output, a.SiteOutput} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("keyless build created an artifact")
				}
			}
		}
	}
}

func TestRunWritesDashboardAndSiteWithCartoKey(t *testing.T) {
	withoutDefaultConfig(t)
	if err := os.MkdirAll("output", 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		"output/places.json":   "[]",
		"output/metadata.json": "{}",
		"output/places.csv":    "name\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const key = "test-key-not-a-credential"
	a := dashboardArgsWithCartoKey(t, key)
	a.BuildSite = true
	if err := run(a); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{a.Output, filepath.Join(a.SiteOutput, "index.html")} {
		page, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if clientDashboardConfig(t, string(page))["cartoApiKey"] != key {
			t.Fatal("built artifact is missing its CARTO key")
		}
	}
}

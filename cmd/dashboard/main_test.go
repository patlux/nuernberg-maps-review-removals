package main

import (
	"os"
	"strings"
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func withoutDefaultConfig(t *testing.T) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestMakeHTMLEscapesCity(t *testing.T) {
	page := makeHTML(args{City: `<script>alert("x")</script>`}, nil)
	if strings.Contains(page, `<script>alert`) {
		t.Fatal("city was inserted as raw HTML")
	}
	if !strings.Contains(page, `&lt;script&gt;alert`) {
		t.Fatal("escaped city is missing from HTML")
	}
}

func TestMakeHTMLCustomCityOmitsNurembergBoundaries(t *testing.T) {
	page := makeHTML(args{City: "Fürth"}, nil)
	if !strings.Contains(page, `<script id="bezirkData" type="application/json">[]</script>`) {
		t.Fatal("custom-city dashboard should not include Nürnberg district boundaries")
	}
}

func TestParseArgsRejectsEmptyCity(t *testing.T) {
	withoutDefaultConfig(t)
	if _, err := parseArgs([]string{"--city"}); err == nil {
		t.Fatal("parseArgs(--city) succeeded, want error")
	}
}

func TestParseArgsRejectsIncompleteLegalConfig(t *testing.T) {
	withoutDefaultConfig(t)
	if _, err := parseArgs([]string{"--legal-name", "Patrick"}); err == nil {
		t.Fatal("parseArgs with incomplete legal config succeeded, want error")
	}
}

func TestParseArgsReadsTOMLConfigAndCLIOverrides(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.toml"
	config := `city = "Nürnberg"
input = "output/places.json"
output = "configured.html"

[legal]
enabled = true
name = "Configured Name"
email = "configured@example.test"
address_lines = [
  "c/o Config GmbH",
  "Configstraße 1",
]
note = "Config note"
post_handler = "Config GmbH"

[analytics]
src = "https://analytics.example.test/js/script.js"
domain = "configured.example.test"
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseArgs([]string{"--config", configPath, "--output", "override.html"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Output != "override.html" {
		t.Fatalf("Output = %q, want CLI override", got.Output)
	}
	if got.LegalName != "Configured Name" || got.LegalEmail != "configured@example.test" || got.LegalPostHandler != "Config GmbH" {
		t.Fatalf("legal config not loaded: %#v", got)
	}
	if got.AnalyticsSrc != "https://analytics.example.test/js/script.js" || got.AnalyticsDomain != "configured.example.test" {
		t.Fatalf("analytics config not loaded: %#v", got)
	}
	if len(got.LegalAddressLines) != 2 || got.LegalAddressLines[0] != "c/o Config GmbH" {
		t.Fatalf("address lines not loaded: %#v", got.LegalAddressLines)
	}
}

func TestParseArgsLegalAddressCLIReplacesConfigLines(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.toml"
	config := `
[legal]
enabled = true
name = "Configured Name"
email = "configured@example.test"
address_lines = ["old line"]
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseArgs([]string{"--config", configPath, "--legal-address-line", "new line 1", "--legal-address-line", "new line 2"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.LegalAddressLines, ",") != "new line 1,new line 2" {
		t.Fatalf("LegalAddressLines = %#v, want CLI lines only", got.LegalAddressLines)
	}
}

func TestMakeClientRowsSkipsRowsWithoutRating(t *testing.T) {
	rows := []mapsreview.Place{
		{ID: "with-rating", Name: "Rated", Rating: mapsreview.FloatPtr(4.5), ReviewCount: mapsreview.IntPtr(10), Status: "success"},
		{ID: "no-rating", Name: "No rating", Rating: nil, ReviewCount: mapsreview.IntPtr(0), Status: "success", PlaceState: mapsreview.PlaceStateNoPublicReviews},
		{ID: "no-count", Name: "No count", Rating: mapsreview.FloatPtr(4.5), Status: "success"},
	}

	got := makeClientRows(rows)
	if len(got) != 1 {
		t.Fatalf("len(makeClientRows) = %d, want 1", len(got))
	}
	if got[0].ID != "with-rating" {
		t.Fatalf("row ID = %q, want with-rating", got[0].ID)
	}
}

func TestMakeHTMLIncludesSEOMetadataAndSummary(t *testing.T) {
	data := []clientRow{{
		ID:              "test-place",
		Name:            "Café <Test>",
		Postcode:        "90402",
		Rating:          mapsreview.FloatPtr(4.5),
		ReviewCount:     mapsreview.IntPtr(120),
		HasBanner:       true,
		RemovedRange:    "21 bis 50",
		RemovedEstimate: 35.5,
		URL:             "https://example.com/maps",
		ReadAt:          "2026-05-04T02:20:07Z",
	}}

	html := makeHTML(args{City: mapsreview.DefaultCity, SiteURL: "https://nuernberg-maps-review-removals.patwoz.dev"}, data)
	checks := []string{
		`<meta name="description" content="Historische Momentaufnahme öffentlich beobachteter Google-Maps-Hinweise im Raum Nürnberg`,
		`<link rel="canonical" href="https://nuernberg-maps-review-removals.patwoz.dev/">`,
		`<meta property="og:type" content="website">`,
		`<script type="application/ld+json">`,
		`<h1 class="hero-title">Nürnberg Google-Maps-Bewertungen</h1>`,
		`Einordnung`,
		`Eigene Bewertung entfernt?`,
		`Google-Einspruch`,
		`EU-Streitbeilegung`,
		`Historische Recherche-Momentaufnahme`,
		`Dashboard basiert auf der MIT-lizenzierten Vorlage von Patrick Wozniak.`,
		`Modellanteile im Vergleich`,
		`ratioHistogram`,
		`Weitere regionale Dashboards`,
		`https://bewertungsradar-saar.de/`,
		`https://berlintrustindex.com/`,
		`Kurzüberblick: Orte mit den höchsten Hinweisspannen`,
		`Café &lt;Test&gt;`,
	}
	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Fatalf("makeHTML missing %q", check)
		}
	}
	for _, unwanted := range []string{`platform-control.com`, `piparo.tech GmbH`, `impressum.html`, `datenschutz.html`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("makeHTML contains unwanted %q", unwanted)
		}
	}
}

func TestMakeHTMLIncludesLegalLinksWhenConfigured(t *testing.T) {
	html := makeHTML(args{
		City:              mapsreview.DefaultCity,
		LegalName:         "Patrick Wozniak",
		LegalEmail:        "hi@example.test",
		LegalAddressLines: []string{"c/o Example GmbH", "Example Street 1", "90443 Nürnberg", "Deutschland"},
		LegalNote:         "Example GmbH is only a delivery address.",
		LegalPostHandler:  "Example GmbH",
	}, nil)
	for _, check := range []string{`methodik.html`, `korrektur.html`, `impressum.html`, `datenschutz.html`} {
		if !strings.Contains(html, check) {
			t.Fatalf("makeHTML missing legal link %q", check)
		}
	}
}

func TestMakeHTMLShowsMapConsentPlaceholder(t *testing.T) {
	html := makeHTML(args{City: mapsreview.DefaultCity}, nil)
	for _, check := range []string{
		`id="mapConsentLoad"`,
		`Karte laden`,
		`Leaflet wird mit dieser Website ausgeliefert`,
		`CARTO`,
	} {
		if !strings.Contains(html, check) {
			t.Fatalf("makeHTML missing map-consent placeholder fragment %q", check)
		}
	}
	beforeScripts := strings.Split(html, `<script id="dashboardConfig"`)[0]
	if strings.Contains(beforeScripts, `Karte wird geladen`) {
		t.Fatal("map should not eagerly show loading text before consent")
	}
}

func TestMakeHTMLAvoidsSuperlativeWorstCaseLabel(t *testing.T) {
	html := makeHTML(args{City: mapsreview.DefaultCity}, nil)
	if strings.Contains(html, "Schlechtestes Worst-Case-Rating") {
		t.Fatal("dashboard still uses the superlative 'Schlechtestes Worst-Case-Rating' label")
	}
	if strings.Contains(html, "Worst-Case") {
		t.Fatal("dashboard still publishes an unsupported synthetic rating")
	}
	if !strings.Contains(html, "Keine Aussage über Betriebsqualität") {
		t.Fatal("dashboard worst-case panel is missing the quality-disclaimer")
	}
}

func TestMakeHTMLUsesNeutralLegendAndPanels(t *testing.T) {
	html := makeHTML(args{City: mapsreview.DefaultCity}, nil)
	for _, want := range []string{
		`Anteil mit Hinweis`,
		`Modellanteil`,
		`Keine gemessene Löschquote; unterschiedliche Zeitbezüge`,
		`Keine Qualitätsaussage gegenüber anderen Orten`,
		`#3c4e6b`,
		`#5b7b8e`,
		`#8a8e94`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("makeHTML missing neutral wording or color %q", want)
		}
	}
	for _, unwanted := range []string{
		`background:#c9332c"></i>hoher Hinweis-Anteil`,
		`background:#ef7d16"></i>sichtbarer Hinweis`,
		`background:#2d7b3f"></i>kein sichtbarer Hinweis`,
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("makeHTML still uses traffic-light legend %q", unwanted)
		}
	}
}

func TestMethodikPageHasHeilberufeNotice(t *testing.T) {
	a := args{
		City:              "Nürnberg",
		LegalName:         "Patrick Wozniak",
		LegalEmail:        "hi@example.test",
		LegalAddressLines: []string{"Example Street 1", "90443 Nürnberg"},
	}
	page := methodikPage(a)
	for _, check := range []string{
		"Heilberufe und personenbezogene Einzelpraxen",
		"§ 19 Abs. 1 Satz 3 MStV",
		"keine Aussage über fachliche Qualität",
		"§ 19 MStV",
		"Journalistische Sorgfalt",
	} {
		if !strings.Contains(page, check) {
			t.Fatalf("methodikPage missing %q", check)
		}
	}
}

func TestKorrekturPageHasGegendarstellung(t *testing.T) {
	a := args{
		LegalName:         "Patrick Wozniak",
		LegalEmail:        "hi@example.test",
		LegalAddressLines: []string{"Example Street 1", "90443 Nürnberg"},
	}
	page := korrekturPage(a)
	for _, check := range []string{
		"Gegendarstellungsanspruch",
		"§ 20 MStV",
		"sechs Wochen",
		"drei Monate",
		"schriftlichen Zugang mit Unterschrift",
	} {
		if !strings.Contains(page, check) {
			t.Fatalf("korrekturPage missing %q", check)
		}
	}
}

func TestDatenschutzPageHasMedienprivileg(t *testing.T) {
	a := args{
		LegalName:         "Patrick Wozniak",
		LegalEmail:        "hi@example.test",
		LegalAddressLines: []string{"Example Street 1", "90443 Nürnberg"},
	}
	page := datenschutzPage(a)
	for _, check := range []string{
		"Medienprivileg",
		"§ 23 MStV",
		"Kapitel V DSGVO",
		"Art. 14 DSGVO",
		"nicht pauschal von der DSGVO",
	} {
		if !strings.Contains(page, check) {
			t.Fatalf("datenschutzPage missing %q", check)
		}
	}
	for _, unwanted := range []string{
		"Transferfolgenabschätzung (TIA)",
		"sinngemäß durchgeführt",
		"Kopie der Standardvertragsklauseln",
		"Modul 2: Verantwortlicher → Auftragsverarbeiter",
	} {
		if strings.Contains(page, unwanted) {
			t.Fatalf("datenschutzPage still claims unsubstantiated %q", unwanted)
		}
	}
}

func TestMapConsentDefaultsToLocalOnly(t *testing.T) {
	page := mapConsentHTML(args{City: mapsreview.DefaultCity})
	for _, want := range []string{"Externe Kartenkacheln sind deaktiviert", "keine Kartenanbieter kontaktiert", "Karte deaktivieren"} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing local-only disclosure %q", want)
		}
	}
	for _, unsupported := range []string{"Art. 49", "DPF-zertifiziert", "unpkg"} {
		if strings.Contains(page, unsupported) {
			t.Fatalf("unsupported provider/transfer claim %q", unsupported)
		}
	}
}

func TestDatenschutzPageMentionsKeyDisclosures(t *testing.T) {
	a := args{
		LegalName:         "Patrick Wozniak",
		LegalEmail:        "hi@example.test",
		LegalAddressLines: []string{"Example Street 1", "90443 Nürnberg"},
	}
	page := datenschutzPage(a)
	for _, check := range []string{
		"GitHub, Inc.",
		"EU-US Data Privacy Framework",
		"kein Plausible-Skript geladen",
		"§ 25 Abs. 2 Nr. 2 TDDDG",
		"Externe Kartenkacheln sind in dieser Ausgabe deaktiviert",
		"Bayerische Landesamt für Datenschutzaufsicht",
		"Art. 77 DSGVO",
	} {
		if !strings.Contains(page, check) {
			t.Fatalf("datenschutzPage missing %q", check)
		}
	}
}

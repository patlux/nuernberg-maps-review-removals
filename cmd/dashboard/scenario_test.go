package main

import (
	"encoding/json"
	"strings"
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func TestDashboardRestoresExplicitOneStarScenario(t *testing.T) {
	page := makeHTML(args{City: "Nürnberg"}, nil)
	for _, want := range []string{
		`<h2>1-Stern-Szenario</h2>`,
		`id="barsScenario"`,
		`data-mode="scenario">1-Stern-Szenario</button>`,
		`data-sort="oneStarScenario"`,
		`id="scenarioExplanation"`,
		`aria-describedby="scenarioExplanation"`,
		`Kein tatsächliches Rating und keine gesicherte Untergrenze`,
		`Spannenmittelpunkt; für „über 250“ der Rechenwert 300`,
		`So sähe die Bewertung rechnerisch aus, wenn die angenommene Zahl entfernter Rezensionen jeweils einen Stern hätte.`,
		`Die tatsächliche Anzahl und Sternebewertung sind unbekannt.`,
		`>1-Stern-Szenario <span class="arrow">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing scenario UI/context %q", want)
		}
	}
	for _, forbidden := range []string{"Worst-Case-Rating", "realRatingAdjusted", "Schlechtestes", "Mathematische Untergrenze", "1-Stern-Szenario (Modell)", "Niedrigste Modellwerte"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("unsupported label %q", forbidden)
		}
	}
}

func TestScenarioInputsSurviveCompactSerialization(t *testing.T) {
	rows := makeClientRows([]mapsreview.Place{{
		Name: "Example", Status: "success", Rating: mapsreview.FloatPtr(4.5), ReviewCount: mapsreview.IntPtr(100),
		HasDefamationNotice: true, RemovedMin: mapsreview.IntPtr(21), RemovedMax: mapsreview.IntPtr(50),
		RealRatingAdjusted: mapsreview.FloatPtr(1.111),
	}})
	var payload compactClientData
	if err := json.Unmarshal([]byte(compactClientDataJSON(rows, "Nürnberg")), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 || len(payload.Rows[0]) != 15 {
		t.Fatalf("unexpected scenario payload: %v", payload.Rows)
	}
	for index, want := range map[int]float64{6: 4.5, 7: 100, 13: 35.5} {
		if got := payload.Rows[0][index]; got != want {
			t.Errorf("input at index %d = %v, want %v", index, got, want)
		}
	}
	if got := payload.Dictionaries[4][0]; got != "21–50" {
		t.Errorf("range = %q", got)
	}
}

func TestMethodologyExplainsScenarioInsteadOfDenyingPublication(t *testing.T) {
	page := methodikPage(args{City: "Nürnberg"})
	for _, want := range []string{"1-Stern-Szenario", "Was-wäre-wenn-Rechnung", "(Google-Rating × sichtbare Rezensionen + Näherungswert × 1)", "keine gesicherte Untergrenze", "3,58", "3,33"} {
		if !strings.Contains(page, want) {
			t.Errorf("missing scenario method %q", want)
		}
	}
	if strings.Contains(page, "Ein berechnetes Ersatz-Sterne-Rating wird nicht veröffentlicht") {
		t.Fatal("methodology contradicts the restored scenario")
	}
}

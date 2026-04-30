package main

import (
	"strings"
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func TestIsRestrictedMapsView(t *testing.T) {
	texts := []string{
		"Die Ansicht ist beschränkt und du siehst nur einen Teil der Google Maps-Daten. Weitere Informationen",
		"This is a limited view and you only see some Google Maps data.",
	}
	for _, text := range texts {
		if !isRestrictedMapsView(text) {
			t.Fatalf("isRestrictedMapsView(%q) = false, want true", text)
		}
	}
	if isRestrictedMapsView("FranKonya\n4,5\n(173)\nRezensionen") {
		t.Fatal("normal place text detected as restricted")
	}
}

func TestParseArgsSaveEvery(t *testing.T) {
	args, err := parseArgs([]string{"--save-every", "25"})
	if err != nil {
		t.Fatal(err)
	}
	if args.SaveEvery != 25 {
		t.Fatalf("SaveEvery = %d, want 25", args.SaveEvery)
	}
}

func TestShouldKeepPreviousRowPreventsBannerClearByDefault(t *testing.T) {
	previous := successPlace()
	previous.HasDefamationNotice = true
	previous.RemovedMin = mapsreview.IntPtr(11)
	previous.RemovedMax = mapsreview.IntPtr(20)
	previous.RemovedEstimate = mapsreview.FloatPtr(15.5)
	previous.RemovedText = mapsreview.StringPtr("11 bis 20 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.")

	next := successPlace()
	keep, reason := shouldKeepPreviousRow(previous, next, true, args{})
	if !keep {
		t.Fatal("shouldKeepPreviousRow = false, want true")
	}
	if !strings.Contains(reason, "clear an existing deletion banner") {
		t.Fatalf("reason = %q, want banner-clear reason", reason)
	}
}

func TestShouldKeepPreviousRowAllowsBannerClearWithFlag(t *testing.T) {
	previous := successPlace()
	previous.HasDefamationNotice = true
	next := successPlace()

	keep, reason := shouldKeepPreviousRow(previous, next, true, args{AllowBannerClears: true})
	if keep {
		t.Fatalf("shouldKeepPreviousRow = true (%q), want false", reason)
	}
}

func TestShouldKeepPreviousRowPreventsStatsRegression(t *testing.T) {
	previous := successPlace()
	next := successPlace()
	next.ReviewCount = nil

	keep, reason := shouldKeepPreviousRow(previous, next, true, args{AllowBannerClears: true})
	if !keep {
		t.Fatal("shouldKeepPreviousRow = false, want true")
	}
	if !strings.Contains(reason, "review count") {
		t.Fatalf("reason = %q, want review-count reason", reason)
	}
}

func successPlace() mapsreview.Place {
	return mapsreview.Place{
		ID:          "place-id",
		Name:        "FranKonya",
		Rating:      mapsreview.FloatPtr(4.5),
		ReviewCount: mapsreview.IntPtr(173),
		Status:      "success",
	}
}

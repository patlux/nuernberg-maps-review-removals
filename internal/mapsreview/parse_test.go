package mapsreview

import (
	"strings"
	"testing"
)

func TestParseNotice(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		min      int
		max      *int
		estimate float64
	}{
		{
			name:     "single word",
			text:     "Eine Bewertung aufgrund einer Beschwerde wegen Diffamierung entfernt.",
			min:      1,
			max:      IntPtr(1),
			estimate: 1,
		},
		{
			name:     "word range",
			text:     "Zwei bis fünf Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.",
			min:      2,
			max:      IntPtr(5),
			estimate: 3.5,
		},
		{
			name:     "numeric range",
			text:     "101 bis 150 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.",
			min:      101,
			max:      IntPtr(150),
			estimate: 125.5,
		},
		{
			name:     "over",
			text:     "Über 250 Bewertungen aufgrund von Beschwerden wegen Diffamierung entfernt.",
			min:      250,
			max:      nil,
			estimate: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notice := ParseNotice(tt.text)
			if notice == nil {
				t.Fatal("notice is nil")
			}
			if notice.Min != tt.min {
				t.Fatalf("min = %d, want %d", notice.Min, tt.min)
			}
			if (notice.Max == nil) != (tt.max == nil) {
				t.Fatalf("max nil = %v, want %v", notice.Max == nil, tt.max == nil)
			}
			if notice.Max != nil && *notice.Max != *tt.max {
				t.Fatalf("max = %d, want %d", *notice.Max, *tt.max)
			}
			if notice.Estimate != tt.estimate {
				t.Fatalf("estimate = %v, want %v", notice.Estimate, tt.estimate)
			}
		})
	}
}

func TestParseNoticeIgnoresUnrelatedText(t *testing.T) {
	if notice := ParseNotice("4,7 Sterne aus 100 Rezensionen"); notice != nil {
		t.Fatalf("notice = %#v, want nil", notice)
	}
}

func TestParsePlaceStatsFromMapsHeader(t *testing.T) {
	stats := ParsePlaceStats("Schnitzery Nürnberg\n4,5\n(268)\nRestaurant\nÜbersicht")
	if stats.Rating == nil || *stats.Rating != 4.5 {
		t.Fatalf("rating = %v, want 4.5", stats.Rating)
	}
	if stats.ReviewCount == nil || *stats.ReviewCount != 268 {
		t.Fatalf("reviewCount = %v, want 268", stats.ReviewCount)
	}
}

func TestParsePlaceStatsPrefersRatingPairedWithReviewCount(t *testing.T) {
	stats := ParsePlaceStats("Route\n1,5 km\nFranKonya\n4,5\n(173)\nCafé mit Frucht- und Süßspeisen")
	if stats.Rating == nil || *stats.Rating != 4.5 {
		t.Fatalf("rating = %v, want 4.5", stats.Rating)
	}
	if stats.ReviewCount == nil || *stats.ReviewCount != 173 {
		t.Fatalf("reviewCount = %v, want 173", stats.ReviewCount)
	}
}

func TestParsePlaceStatsAcceptsNonBreakingSpaceBeforeStars(t *testing.T) {
	stats := ParsePlaceStats("FranKonya\n4,5\u00a0Sterne \nWeitere Informationen")
	if stats.Rating == nil || *stats.Rating != 4.5 {
		t.Fatalf("rating = %v, want 4.5", stats.Rating)
	}
}

func TestParsePlaceStatsFromCompactLightpandaReviewsText(t *testing.T) {
	stats := ParsePlaceStats("543214,5173 Berichte Rezensionen werden nicht überprüft max. 1,5 Sterne")
	if stats.Rating == nil || *stats.Rating != 4.5 {
		t.Fatalf("rating = %v, want 4.5", stats.Rating)
	}
	if stats.ReviewCount == nil || *stats.ReviewCount != 173 {
		t.Fatalf("reviewCount = %v, want 173", stats.ReviewCount)
	}
}

func TestParsePlaceStatsDoesNotTreatReviewCountAsCompactRating(t *testing.T) {
	stats := ParsePlaceStats("4,8\n2.336 Berichte Rezensionen werden nicht überprüft")
	if stats.Rating == nil || *stats.Rating != 4.8 {
		t.Fatalf("rating = %v, want 4.8", stats.Rating)
	}
	if stats.ReviewCount == nil || *stats.ReviewCount != 2336 {
		t.Fatalf("reviewCount = %v, want 2336", stats.ReviewCount)
	}
}

func TestParsePlaceStatsIgnoresSuggestedPlacesSection(t *testing.T) {
	stats := ParsePlaceStats("EAT HAPPY\nÜbersicht\nInfo\nRoutenplaner\nRezension schreiben\nWird auch oft gesucht\nHaDaCo Sushi Thai Wok\n4,5(105)")
	if stats.Rating != nil || stats.ReviewCount != nil {
		t.Fatalf("stats = %#v, want nil rating/reviewCount", stats)
	}
}

func TestParsePlaceStatsIgnoresReviewerProfileCounts(t *testing.T) {
	stats := ParsePlaceStats("EAT HAPPY\n5,0\n1 Rezension\nRezensionen werden nicht überprüft\nIsabella Staudenmaier Local Guide · 621 Rezensionen · 5.064 Fotos")
	if stats.ReviewCount == nil || *stats.ReviewCount != 1 {
		t.Fatalf("reviewCount = %v, want 1", stats.ReviewCount)
	}
}

func TestParsePlaceStatsIgnoresReviewerProfileCountsWithoutVerifiedMarker(t *testing.T) {
	stats := ParsePlaceStats("EAT HAPPY\n5,0 Sterne\n1 Rezension\nIsabella Staudenmaier Local Guide · 621 Rezensionen · 5.064 Fotos")
	if stats.ReviewCount == nil || *stats.ReviewCount != 1 {
		t.Fatalf("reviewCount = %v, want 1", stats.ReviewCount)
	}
}

func TestParsePlaceStatsDropsReviewCountWithoutRating(t *testing.T) {
	stats := ParsePlaceStats("Irgendwas\n621 Berichte")
	if stats.Rating != nil || stats.ReviewCount != nil {
		t.Fatalf("stats = %#v, want nil rating/reviewCount", stats)
	}
}

func TestExtractAddressIgnoresSuggestedPlacesSection(t *testing.T) {
	address := ExtractAddress("EAT HAPPY\nWird auch oft gesucht\nHaDaCo Sushi Thai Wok\nKirchenweg 5, 90419 Nürnberg")
	if address != nil {
		t.Fatalf("address = %v, want nil", *address)
	}
}

func TestParseGermanNumber(t *testing.T) {
	tests := map[string]float64{
		"1.234": 1234,
		"4,7":   4.7,
		"250":   250,
	}
	for input, want := range tests {
		got, ok := ParseGermanNumber(input)
		if !ok || got != want {
			t.Fatalf("ParseGermanNumber(%q) = %v, %v; want %v, true", input, got, ok, want)
		}
	}
}

func TestReviewsURLFromURL(t *testing.T) {
	raw := "https://www.google.com/maps/place/FranKonya/data=!4m7!3m6!1s0x479f57a73350aed5:0xef0321790f9cee83!8m2!3d49.4471632!4d11.0647079!16s%2Fg%2F11t1h2jrkw!19sChIJ1a5QM6dXn0cRg-6cD3khA-8?authuser=0&hl=de&rclk=1"
	got := ReviewsURLFromURL(raw)
	if !strings.Contains(got, "!9m1!1b1!16s") {
		t.Fatalf("reviews URL %q does not contain reviews tab marker", got)
	}
	if strings.Contains(got, "!19s") {
		t.Fatalf("reviews URL %q still contains search-result marker", got)
	}
	if !strings.Contains(got, "hl=de") {
		t.Fatalf("reviews URL %q lost query", got)
	}
}

func TestReviewsURLFromURLCanonicalizesAtDataURLs(t *testing.T) {
	raw := "https://www.google.com/maps/place/Schnitzery+N%C3%BCrnberg/@49.449978,11.0735199,17z/data=!3m1!4b1!4m6!3m5!1s0x479f5793537290f3:0x369418ec26602e09!8m2!3d49.449978!4d11.0735199!16s%2Fg%2F11lyrycx33?entry=ttu&hl=de"
	got := ReviewsURLFromURL(raw)
	if strings.Contains(got, "/@") {
		t.Fatalf("reviews URL %q still contains @ coordinate path", got)
	}
	if strings.Contains(got, "!3m1!4b1") {
		t.Fatalf("reviews URL %q still contains map viewport data prefix", got)
	}
	if !strings.Contains(got, "/data=!4m8!3m7") || !strings.Contains(got, "!9m1!1b1!16s") {
		t.Fatalf("reviews URL %q was not converted to canonical reviews data URL", got)
	}
}

func TestReviewsURLFromURLIncrementsExistingDataCounts(t *testing.T) {
	raw := "https://www.google.com/maps/place/Das+Steichele/data=!4m10!3m9!1s0x479f57a9a65ab759:0x123dd70a4e8f0ed0!5m2!4m1!1i2!8m2!3d49.449225!4d11.071107!16s%2Fg%2F126122ghp!19sSearchResult?authuser=0&hl=de&rclk=1"
	got := ReviewsURLFromURL(raw)
	if !strings.Contains(got, "!4m11!3m10") {
		t.Fatalf("reviews URL %q did not increment data counts", got)
	}
	if !strings.Contains(got, "!9m1!1b1!16s") {
		t.Fatalf("reviews URL %q does not contain reviews tab marker", got)
	}
}

func TestExtractCoordinates(t *testing.T) {
	tests := []string{
		"https://www.google.com/maps/place/Foo/data=!4m7!3m6!1sabc!8m2!3d49.4521!4d11.0767!16sbar",
		"https://www.google.com/maps/place/Foo/@49.4521,11.0767,17z",
	}
	for _, input := range tests {
		coords := ExtractCoordinates(input)
		if coords == nil {
			t.Fatalf("ExtractCoordinates(%q) = nil", input)
		}
		if coords.Lat != 49.4521 || coords.Lng != 11.0767 {
			t.Fatalf("coords = %#v, want 49.4521/11.0767", coords)
		}
	}
}

func TestComputeMetrics(t *testing.T) {
	row := Place{Rating: FloatPtr(4.5), ReviewCount: IntPtr(100), RemovedEstimate: FloatPtr(25)}
	ComputeMetrics(&row)
	if row.DeletionRatioPct == nil || *row.DeletionRatioPct != 20 {
		t.Fatalf("deletionRatioPct = %v, want 20", row.DeletionRatioPct)
	}
	if row.RealRatingAdjusted == nil || *row.RealRatingAdjusted != 3.8 {
		t.Fatalf("realRatingAdjusted = %v, want 3.8", row.RealRatingAdjusted)
	}
}

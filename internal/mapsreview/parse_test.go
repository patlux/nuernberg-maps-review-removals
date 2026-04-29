package mapsreview

import "testing"

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

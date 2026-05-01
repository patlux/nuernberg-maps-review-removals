package main

import (
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func TestMakeClientRowsSkipsRowsWithoutRating(t *testing.T) {
	rows := []mapsreview.Place{
		{ID: "with-rating", Name: "Rated", Rating: mapsreview.FloatPtr(4.5), ReviewCount: mapsreview.IntPtr(10), Status: "success"},
		{ID: "no-rating", Name: "No rating", Rating: nil, ReviewCount: mapsreview.IntPtr(0), Status: "success", PlaceState: mapsreview.PlaceStateNoPublicReviews},
	}

	got := makeClientRows(rows)
	if len(got) != 1 {
		t.Fatalf("len(makeClientRows) = %d, want 1", len(got))
	}
	if got[0].ID != "with-rating" {
		t.Fatalf("row ID = %q, want with-rating", got[0].ID)
	}
}

package mapsreview

import "testing"

func TestApplyPlaceOverrides(t *testing.T) {
	row := Place{
		ID:          "0x479f5793537290f3:0x369418ec26602e09",
		Name:        "Schnitzery Nürnberg",
		Rating:      FloatPtr(4.5),
		ReviewCount: IntPtr(268),
		Status:      "success",
	}
	ApplyPlaceOverrides(&row)
	if !row.HasDefamationNotice {
		t.Fatal("expected override to mark row as having a defamation notice")
	}
	if row.RemovedMin == nil || *row.RemovedMin != 51 || row.RemovedMax == nil || *row.RemovedMax != 100 {
		t.Fatalf("unexpected removal range: min=%v max=%v", row.RemovedMin, row.RemovedMax)
	}
	if row.DeletionRatioPct == nil || *row.DeletionRatioPct != 21.98 {
		t.Fatalf("unexpected deletion ratio: %v", row.DeletionRatioPct)
	}
}

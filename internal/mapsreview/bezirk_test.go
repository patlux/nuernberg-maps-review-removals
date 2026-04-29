package mapsreview

import "testing"

func TestAssignBezirkExact(t *testing.T) {
	bezirk := AssignBezirk(49.4505, 11.0786)
	if bezirk == nil {
		t.Fatal("expected a Bezirk for central Nürnberg coordinates")
	}
	if bezirk.ID != "01" || bezirk.Name != "Altstadt, St. Lorenz" {
		t.Fatalf("unexpected Bezirk: %#v", bezirk)
	}
}

func TestAssignBezirkFallbackForNurembergPostcode(t *testing.T) {
	bezirk := AssignBezirkForPostcode(49.4486432, 11.0777619, "90402")
	if bezirk == nil {
		t.Fatal("expected fallback Bezirk assignment inside Nürnberg")
	}
	if bezirk.ID != "01" || bezirk.Name != "Altstadt, St. Lorenz" {
		t.Fatalf("unexpected fallback Bezirk: %#v", bezirk)
	}
}

func TestAssignBezirkDoesNotFallbackOutsideNuremberg(t *testing.T) {
	bezirk := AssignBezirkForPostcode(49.4771, 10.9887, "90762")
	if bezirk != nil {
		t.Fatalf("expected no Bezirk outside Nürnberg, got %#v", bezirk)
	}
}

func TestBezirkBoundaries(t *testing.T) {
	boundaries := BezirkBoundaries()
	if len(boundaries) == 0 {
		t.Fatal("expected Bezirk map boundaries")
	}
	first := boundaries[0]
	if first.ID == "" || first.Label == "" || len(first.Polygons) == 0 || len(first.Polygons[0]) == 0 {
		t.Fatalf("unexpected first boundary: %#v", first)
	}
	lat := first.Polygons[0][0][0]
	lng := first.Polygons[0][0][1]
	if lat < 49 || lat > 50 || lng < 10 || lng > 12 {
		t.Fatalf("boundary coordinate outside Nürnberg region: lat=%f lng=%f", lat, lng)
	}
}

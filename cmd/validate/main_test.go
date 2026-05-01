package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuernberg-maps-review-removals/internal/mapsreview"
)

func TestParseArgsValidateDefaults(t *testing.T) {
	args, err := parseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if args.Input != mapsreview.ResultsJSON {
		t.Fatalf("Input = %q, want %q", args.Input, mapsreview.ResultsJSON)
	}
	if args.StrictNuremberg {
		t.Fatal("StrictNuremberg should default to false")
	}
}

func TestParseArgsValidateStrict(t *testing.T) {
	args, err := parseArgs([]string{"--strict-nuremberg"})
	if err != nil {
		t.Fatal(err)
	}
	if !args.StrictNuremberg {
		t.Fatal("StrictNuremberg should be true")
	}
}

func TestValidateSuccessRows(t *testing.T) {
	rows := []mapsreview.Place{
		{
			ID:          "test-id-1",
			Name:        "Test Place",
			URL:         "https://www.google.com/maps/place/Test+Place/@49.4521,11.0767,17z",
			Postcode:    mapsreview.StringPtr("90402"),
			Address:     mapsreview.StringPtr("Teststraße 1, 90402 Nürnberg"),
			Rating:      mapsreview.FloatPtr(4.5),
			ReviewCount: mapsreview.IntPtr(100),
			Lat:         mapsreview.FloatPtr(49.4521),
			Lng:         mapsreview.FloatPtr(11.0767),
			BezirkID:    mapsreview.StringPtr("01"),
			BezirkName:  mapsreview.StringPtr("Altstadt, St. Lorenz"),
			Status:      "success",
		},
	}

	tmpFile := writeTempPlacesJSON(t, rows)
	defer os.Remove(tmpFile)

	args := args{Input: tmpFile}
	err := run(args)
	if err != nil {
		t.Fatalf("validation failed for valid data: %v", err)
	}
}

func TestValidateDetectsDuplicates(t *testing.T) {
	rows := []mapsreview.Place{
		{
			ID:          "dup-id",
			Name:        "A",
			URL:         "https://maps.example.com/A",
			Rating:      mapsreview.FloatPtr(4.0),
			ReviewCount: mapsreview.IntPtr(10),
			Postcode:    mapsreview.StringPtr("90402"),
			Address:     mapsreview.StringPtr("Str 1, 90402 Nürnberg"),
			Status:      "success",
		},
		{
			ID:          "dup-id",
			Name:        "B",
			URL:         "https://maps.example.com/B",
			Rating:      mapsreview.FloatPtr(4.0),
			ReviewCount: mapsreview.IntPtr(10),
			Postcode:    mapsreview.StringPtr("90402"),
			Address:     mapsreview.StringPtr("Str 2, 90402 Nürnberg"),
			Status:      "success",
		},
	}

	tmpFile := writeTempPlacesJSON(t, rows)
	defer os.Remove(tmpFile)

	args := args{Input: tmpFile}
	err := run(args)
	if err == nil {
		t.Fatal("expected validation error for duplicate IDs")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message does not indicate validation failure: %v", err)
	}
}

func TestValidateOutsidePostcodeWarning(t *testing.T) {
	rows := []mapsreview.Place{
		{
			ID:          "outside-id",
			Name:        "Outside Place",
			URL:         "https://maps.example.com/Outside",
			Postcode:    mapsreview.StringPtr("80331"),
			Rating:      mapsreview.FloatPtr(4.0),
			ReviewCount: mapsreview.IntPtr(10),
			Status:      "success",
		},
	}

	tmpFile := writeTempPlacesJSON(t, rows)
	defer os.Remove(tmpFile)

	// Outside postcode is a warning, not an error (unless --strict-nuremberg)
	args := args{Input: tmpFile}
	err := run(args)
	if err != nil {
		t.Fatalf("outside postcode should be warning, not error: %v", err)
	}
}

func TestValidateStrictNurembergErrorsOnOutside(t *testing.T) {
	rows := []mapsreview.Place{
		{
			ID:          "outside-id",
			Name:        "Outside Place",
			URL:         "https://maps.example.com/Outside",
			Postcode:    mapsreview.StringPtr("80331"),
			Rating:      mapsreview.FloatPtr(4.0),
			ReviewCount: mapsreview.IntPtr(10),
			Status:      "success",
		},
	}

	tmpFile := writeTempPlacesJSON(t, rows)
	defer os.Remove(tmpFile)

	args := args{Input: tmpFile, StrictNuremberg: true}
	err := run(args)
	if err == nil {
		t.Fatal("expected validation error with --strict-nuremberg")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message does not indicate validation failure: %v", err)
	}
}

func writeTempPlacesJSON(t *testing.T, rows []mapsreview.Place) string {
	t.Helper()
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "places.json")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

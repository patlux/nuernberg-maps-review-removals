package mapsreview

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed data/place_overrides.json
var placeOverrideData []byte

type placeOverride struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Rating      *float64 `json:"rating,omitempty"`
	ReviewCount *int     `json:"reviewCount,omitempty"`
	RemovedText *string  `json:"removedText,omitempty"`
	Source      string   `json:"source,omitempty"`
}

var (
	placeOverrideOnce   sync.Once
	placeOverrideByID   map[string]placeOverride
	placeOverrideByName map[string]placeOverride
)

func ApplyPlaceOverrides(row *Place) {
	if row == nil {
		return
	}
	override, ok := lookupPlaceOverride(row)
	if !ok {
		return
	}
	if override.Rating != nil {
		row.Rating = FloatPtr(*override.Rating)
	}
	if override.ReviewCount != nil {
		row.ReviewCount = IntPtr(*override.ReviewCount)
	}
	if override.RemovedText != nil && *override.RemovedText != "" {
		if notice := ParseNotice(*override.RemovedText); notice != nil {
			row.HasDefamationNotice = true
			row.RemovedMin = IntPtr(notice.Min)
			row.RemovedMax = notice.Max
			row.RemovedEstimate = FloatPtr(notice.Estimate)
			row.RemovedText = StringPtr(notice.Text)
		}
	}
	ComputeMetrics(row)
}

func lookupPlaceOverride(row *Place) (placeOverride, bool) {
	loadPlaceOverrides()
	if row.ID != "" {
		if override, ok := placeOverrideByID[row.ID]; ok {
			return override, true
		}
	}
	if row.Name != "" {
		if override, ok := placeOverrideByName[row.Name]; ok {
			return override, true
		}
	}
	return placeOverride{}, false
}

func loadPlaceOverrides() {
	placeOverrideOnce.Do(func() {
		placeOverrideByID = map[string]placeOverride{}
		placeOverrideByName = map[string]placeOverride{}
		var overrides []placeOverride
		if err := json.Unmarshal(placeOverrideData, &overrides); err != nil {
			return
		}
		for _, override := range overrides {
			if override.ID != "" {
				placeOverrideByID[override.ID] = override
			}
			if override.Name != "" {
				placeOverrideByName[override.Name] = override
			}
		}
	})
}

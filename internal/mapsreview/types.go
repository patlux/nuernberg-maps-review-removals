package mapsreview

import "time"

const (
	PlaceStateActive            = "active"
	PlaceStateNoPublicReviews   = "no_public_reviews"
	PlaceStatePermanentlyClosed = "permanently_closed"
	PlaceStateTemporarilyClosed = "temporarily_closed"
	PlaceStatePartialLoad       = "partial_load"
)

type Discovery struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	DiscoveredPostcode string `json:"discoveredPostcode,omitempty"`
	DiscoveredQuery    string `json:"discoveredQuery,omitempty"`
}

type Place struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Postcode            *string  `json:"postcode"`
	Address             *string  `json:"address"`
	Rating              *float64 `json:"rating"`
	ReviewCount         *int     `json:"reviewCount"`
	Category            *string  `json:"category"`
	Lat                 *float64 `json:"lat,omitempty"`
	Lng                 *float64 `json:"lng,omitempty"`
	BezirkID            *string  `json:"bezirkId,omitempty"`
	BezirkName          *string  `json:"bezirkName,omitempty"`
	HasDefamationNotice bool     `json:"hasDefamationNotice"`
	RemovedMin          *int     `json:"removedMin"`
	RemovedMax          *int     `json:"removedMax"`
	RemovedEstimate     *float64 `json:"removedEstimate"`
	DeletionRatioPct    *float64 `json:"deletionRatioPct"`
	RealRatingAdjusted  *float64 `json:"realRatingAdjusted"`
	RemovedText         *string  `json:"removedText"`
	URL                 string   `json:"url"`
	ReadAt              string   `json:"readAt"`
	PlaceState          string   `json:"placeState,omitempty"`
	Status              string   `json:"status"`
	Error               *string  `json:"error"`
}

type Notice struct {
	Text     string
	Min      int
	Max      *int
	Estimate float64
}

type PlaceStats struct {
	Rating      *float64
	ReviewCount *int
}

func StringPtr(value string) *string  { return &value }
func IntPtr(value int) *int           { return &value }
func FloatPtr(value float64) *float64 { return &value }

func StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func IntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func FloatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

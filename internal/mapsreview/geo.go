package mapsreview

import (
	"net/url"
	"regexp"
	"strconv"
)

type Coordinates struct {
	Lat float64
	Lng float64
}

func ExtractCoordinates(rawURL string) *Coordinates {
	decoded, err := url.QueryUnescape(rawURL)
	if err != nil {
		decoded = rawURL
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`!3d([-0-9.]+)!4d([-0-9.]+)`),
		regexp.MustCompile(`@([-0-9.]+),([-0-9.]+)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(decoded)
		if len(match) < 3 {
			continue
		}
		lat, latErr := strconv.ParseFloat(match[1], 64)
		lng, lngErr := strconv.ParseFloat(match[2], 64)
		if latErr == nil && lngErr == nil && validCoordinate(lat, lng) {
			return &Coordinates{Lat: lat, Lng: lng}
		}
	}
	return nil
}

func validCoordinate(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

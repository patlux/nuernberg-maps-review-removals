package mapsreview

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func NormalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("hl", "de")
	u.RawQuery = q.Encode()
	return u.String()
}

func PlaceIDFromURL(raw string) string {
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	if match := regexp.MustCompile(`!1s([^!]+)`).FindStringSubmatch(decoded); len(match) > 1 {
		return match[1]
	}
	place := "place"
	if match := regexp.MustCompile(`/maps/place/([^/@?]+)`).FindStringSubmatch(decoded); len(match) > 1 {
		place = match[1]
	}
	coords := raw
	if match := regexp.MustCompile(`@([-0-9.]+,[-0-9.]+)`).FindStringSubmatch(decoded); len(match) > 1 {
		coords = match[1]
	}
	return place + ":" + coords
}

func ParseGermanNumber(value string) (float64, bool) {
	s := strings.TrimSpace(value)
	if s == "" {
		return 0, false
	}
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func ParseNotice(text string) *Notice {
	compact := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(compact)
	if !strings.Contains(lower, "diffamierung") || !strings.Contains(lower, "entfernt") {
		return nil
	}

	numberToken := `(eine|ein|einer|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn|\d+[.]?\d*)`
	notice := firstMatch(compact,
		`(?i)((?:über|mehr als)\s+\d+[.]?\d*\s+Bewertung(?:en)?.{0,160}?Diffamierung.{0,80}?entfernt\.?)`,
		`(?i)(`+numberToken+`\s+(?:bis\s+`+numberToken+`\s+)?Bewertung(?:en)?.{0,160}?Diffamierung.{0,80}?entfernt\.?)`,
		`(?i)(.{0,80}Diffamierung.{0,80}entfernt\.?)`,
	)
	if notice == "" {
		return nil
	}

	var min *int
	var max *int
	if match := regexp.MustCompile(`(?i)(?:über|mehr als)\s+(\d+[.]?\d*)\s+Bewertung(?:en)?`).FindStringSubmatch(notice); len(match) > 1 {
		if n, ok := numberFromToken(match[1]); ok {
			min = IntPtr(n)
		}
	}

	rangeRe := regexp.MustCompile(`(?i)` + numberToken + `\s+bis\s+` + numberToken + `\s+Bewertung(?:en)?`)
	if match := rangeRe.FindStringSubmatch(notice); len(match) > 2 {
		if a, ok := numberFromToken(match[1]); ok {
			if b, ok := numberFromToken(match[2]); ok {
				min = IntPtr(a)
				max = IntPtr(b)
			}
		}
	}

	if min == nil {
		singleRe := regexp.MustCompile(`(?i)` + numberToken + `\s+Bewertung(?:en)?`)
		if match := singleRe.FindStringSubmatch(notice); len(match) > 1 {
			if n, ok := numberFromToken(match[1]); ok {
				min = IntPtr(n)
				max = IntPtr(n)
			}
		}
	}

	if min == nil && regexp.MustCompile(`(?i)eine\s+bewertung`).MatchString(notice) {
		min = IntPtr(1)
		max = IntPtr(1)
	}
	if min == nil {
		return nil
	}

	estimate := float64(*min) + 50
	if max != nil {
		estimate = (float64(*min) + float64(*max)) / 2
	}
	return &Notice{Text: strings.TrimSpace(notice), Min: *min, Max: max, Estimate: estimate}
}

func firstMatch(text string, patterns ...string) string {
	for _, pattern := range patterns {
		match := regexp.MustCompile(pattern).FindStringSubmatch(text)
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func numberFromToken(token string) (int, bool) {
	words := map[string]int{
		"eine": 1, "ein": 1, "einer": 1, "zwei": 2, "drei": 3, "vier": 4,
		"fünf": 5, "fuenf": 5, "sechs": 6, "sieben": 7, "acht": 8,
		"neun": 9, "zehn": 10,
	}
	lower := strings.ToLower(strings.TrimSpace(token))
	if n, ok := words[lower]; ok {
		return n, true
	}
	normalized := strings.ReplaceAll(lower, "ü", "ue")
	if n, ok := words[normalized]; ok {
		return n, true
	}
	if n, ok := ParseGermanNumber(token); ok {
		return int(n), true
	}
	return 0, false
}

func ParsePlaceStats(text string) PlaceStats {
	ratingPatterns := []string{
		`(?i)(?:^|\n|\s)([1-5][,.][0-9])\s*(?:Sterne|stars|★)`,
		`(?i)(?:^|\n|\s)([1-5][,.][0-9])\s*\n\s*(?:\(?[\d.]+\)?\s*)?(?:Rezensionen|Berichte)`,
		`(?i)\b([1-5][,.][0-9])\b`,
	}
	var rating *float64
	for _, pattern := range ratingPatterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			if n, ok := ParseGermanNumber(match[1]); ok {
				rating = FloatPtr(n)
				break
			}
		}
	}

	var reviewCount *int
	reviewRe := regexp.MustCompile(`(?i)(?:^|\n|\s|\()([0-9][0-9.]*)\)?\s*(?:Rezensionen|Berichte)\b`)
	for _, match := range reviewRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			if n, ok := ParseGermanNumber(match[1]); ok && n >= 0 {
				reviewCount = IntPtr(int(n))
				break
			}
		}
	}
	return PlaceStats{Rating: rating, ReviewCount: reviewCount}
}

func ExtractAddress(text string) *string {
	patterns := []string{
		`(?i)Adresse:\s*([^\n]*\b9\d{4}\s+[^\n]*)`,
		`(?i)\n([^\n]*,\s*9\d{4}\s+[^\n]*)\n`,
	}
	for _, pattern := range patterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			address := strings.TrimSpace(regexp.MustCompile(`(?i)^Adresse:\s*`).ReplaceAllString(match[1], ""))
			if address != "" {
				return StringPtr(address)
			}
		}
	}
	return nil
}

func ExtractPostcode(text string) *string {
	if match := regexp.MustCompile(`\b9\d{4}\b`).FindStringSubmatch(text); len(match) > 0 {
		return StringPtr(match[0])
	}
	return nil
}

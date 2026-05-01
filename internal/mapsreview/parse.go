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

func ReviewsURLFromURL(raw string) string {
	base, query, hasQuery := strings.Cut(NormalizeURL(raw), "?")
	base = canonicalizeMapsDataBase(base)
	hasSearchResultMarker := regexp.MustCompile(`!19s[^!/?]+`).MatchString(base)
	if !strings.Contains(base, "!9m1!1b1") {
		delta := 2
		if hasSearchResultMarker {
			delta = 1
		}
		base = incrementMapsDataCounts(base, delta)
		base = strings.Replace(base, "!16s", "!9m1!1b1!16s", 1)
	}
	base = regexp.MustCompile(`!19s[^!/?]+`).ReplaceAllString(base, "")
	if hasQuery {
		return base + "?" + query
	}
	return base
}

func canonicalizeMapsDataBase(base string) string {
	if strings.Contains(base, "/@") && strings.Contains(base, "/data=") {
		base = regexp.MustCompile(`/@[^/]+/data=`).ReplaceAllString(base, "/data=")
	}
	return regexp.MustCompile(`/data=!3m1!4b1(!4m)`).ReplaceAllString(base, `/data=$1`)
}

func incrementMapsDataCounts(base string, delta int) string {
	match := regexp.MustCompile(`!4m(\d+)!3m(\d+)`).FindStringSubmatch(base)
	if len(match) != 3 {
		return base
	}
	outer, outerErr := strconv.Atoi(match[1])
	inner, innerErr := strconv.Atoi(match[2])
	if outerErr != nil || innerErr != nil {
		return base
	}
	return strings.Replace(base, match[0], "!4m"+strconv.Itoa(outer+delta)+"!3m"+strconv.Itoa(inner+delta), 1)
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
	var rating *float64
	var reviewCount *int
	compactRatingReviewRe := regexp.MustCompile(`(?i)([1-5][,.][0-9])([0-9][0-9.]*)[\s\x{00a0}]*(?:Rezensionen|Berichte)\b`)
	if match := compactRatingReviewRe.FindStringSubmatch(text); len(match) > 2 {
		if n, ok := ParseGermanNumber(match[1]); ok && n >= 1 && n <= 5 {
			rating = FloatPtr(n)
			if count, ok := ParseGermanNumber(match[2]); ok && count >= 0 {
				reviewCount = IntPtr(int(count))
			}
		}
	}
	reviewPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:^|\n|\s|\x{00a0}|\()([0-9][0-9.]*)\)?[\s\x{00a0}]*(?:Rezensionen|Berichte)\b`),
		regexp.MustCompile(`(?m)^[1-5][,.][0-9]\s*\n\s*\(([0-9][0-9.]*)\)\s*$`),
	}
	for _, reviewRe := range reviewPatterns {
		if reviewCount != nil {
			break
		}
		for _, match := range reviewRe.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 {
				if n, ok := ParseGermanNumber(match[1]); ok && n >= 0 {
					reviewCount = IntPtr(int(n))
					break
				}
			}
		}
		if reviewCount != nil {
			break
		}
	}

	if reviewCount != nil && rating == nil {
		if n, ok := parseRatingNearReviewCount(text, *reviewCount); ok {
			rating = FloatPtr(n)
		}
	}
	if rating == nil {
		ratingText := text
		for _, marker := range []string{"Rezensionen werden nicht überprüft", "Reviews are not verified", "Alle Rezensionen", "Sortieren"} {
			if idx := strings.Index(strings.ToLower(ratingText), strings.ToLower(marker)); idx >= 0 {
				ratingText = ratingText[:idx]
				break
			}
		}
		ratingPatterns := []string{
			`(?i)(?:^|\n|\s)([1-5][,.][0-9])[\s\x{00a0}]*(?:Sterne|stars|★)`,
			`(?m)(?:^|\n)\s*([1-5][,.][0-9])\s*\n\s*\([0-9][0-9.]*\)\s*(?:\n|$)`,
		}
		for _, pattern := range ratingPatterns {
			if match := regexp.MustCompile(pattern).FindStringSubmatch(ratingText); len(match) > 1 {
				if n, ok := ParseGermanNumber(match[1]); ok {
					rating = FloatPtr(n)
					break
				}
			}
		}
	}
	return PlaceStats{Rating: rating, ReviewCount: reviewCount}
}

func parseRatingNearReviewCount(text string, reviewCount int) (float64, bool) {
	countPattern := regexp.QuoteMeta(strconv.Itoa(reviewCount))
	if reviewCount >= 1000 {
		formatted := strconv.Itoa(reviewCount)
		parts := []string{}
		for len(formatted) > 3 {
			parts = append([]string{formatted[len(formatted)-3:]}, parts...)
			formatted = formatted[:len(formatted)-3]
		}
		parts = append([]string{formatted}, parts...)
		countPattern = regexp.QuoteMeta(strings.Join(parts, ".")) + `|` + countPattern
	}
	patterns := []string{
		`(?i)([1-5][,.][0-9])[\s\x{00a0}]*(?:Sterne|stars|★)?\s*\n\s*\(?(?:` + countPattern + `)\)?\s*(?:Rezensionen|Berichte)?\b`,
		`(?i)([1-5][,.][0-9])[\s\x{00a0}]*(?:Sterne|stars|★).{0,80}(?:` + countPattern + `)\s*(?:Rezensionen|Berichte)`,
	}
	for _, pattern := range patterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			if n, ok := ParseGermanNumber(match[1]); ok {
				return n, true
			}
		}
	}
	return 0, false
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

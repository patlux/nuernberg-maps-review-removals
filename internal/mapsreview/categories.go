package mapsreview

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/category_rules.json
var categoryRulesJSON []byte

// CategoryRules defines the configurable category normalization rules.
type CategoryRules struct {
	BlockedStrings []string         `json:"blocked_strings"`
	ReviewKeywords []string         `json:"review_keywords"`
	NameKeywords   []string         `json:"name_keywords"`
	Categories     []CategoryBucket `json:"categories"`
}

// CategoryBucket is one canonical category with its matching keywords.
// Keywords are matched case-insensitively as substrings.
// Categories are checked in order; the first match wins.
// The last entry should have empty keywords as catch-all.
type CategoryBucket struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

var (
	categoryRules     *CategoryRules
	categoryRulesOnce sync.Once
)

// LoadCategoryRules returns the parsed category rules, loading from the
// embedded JSON on first call.
func LoadCategoryRules() *CategoryRules {
	categoryRulesOnce.Do(func() {
		var rules CategoryRules
		if err := json.Unmarshal(categoryRulesJSON, &rules); err != nil {
			panic("failed to parse embedded category_rules.json: " + err.Error())
		}
		// Build blocked set for O(1) lookup
		categoryRules = &rules
	})
	return categoryRules
}

// NormalizeCategory maps a raw category string to a canonical bucket using the
// rules from category_rules.json. Returns empty string if the input is empty.
func NormalizeCategory(candidate string) string {
	if candidate == "" {
		return ""
	}
	rules := LoadCategoryRules()
	lower := strings.ToLower(candidate)
	for _, bucket := range rules.Categories {
		if len(bucket.Keywords) == 0 {
			return bucket.Name // catch-all
		}
		for _, kw := range bucket.Keywords {
			if strings.Contains(lower, kw) {
				return bucket.Name
			}
		}
	}
	return rules.Categories[len(rules.Categories)-1].Name
}

// BlockedCategorySet returns a set of exact-match strings that should be
// rejected as non-categories (Google Maps UI elements).
func BlockedCategorySet() map[string]bool {
	rules := LoadCategoryRules()
	set := make(map[string]bool, len(rules.BlockedStrings))
	for _, s := range rules.BlockedStrings {
		set[s] = true
	}
	return set
}

// ReviewKeywords returns the list of keywords used to detect review snippets.
func ReviewKeywords() []string {
	return LoadCategoryRules().ReviewKeywords
}

// NameKeywords returns the list of keywords that indicate a place name
// contains a category keyword (for name-based inference fallback).
func NameKeywords() []string {
	return LoadCategoryRules().NameKeywords
}

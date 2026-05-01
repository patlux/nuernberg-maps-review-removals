package mapsreview

import (
	"strconv"
	"strings"
)

// SplitArg parses a --key=value or --key value pair from argv starting at index.
// It returns the key, value, and whether the value consumed the next argv element.
func SplitArg(argv []string, index int) (key string, value string, consume bool) {
	arg := argv[index]
	if before, after, ok := strings.Cut(arg, "="); ok {
		return before, after, false
	}
	if index+1 < len(argv) && !strings.HasPrefix(argv[index+1], "--") {
		return arg, argv[index+1], true
	}
	return arg, "", false
}

// Atoi parses a string to int, returning 0 on parse failure.
func Atoi(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

// ParseBool parses a boolean string, returning def when value is empty.
func ParseBool(value string, def bool) bool {
	if value == "" {
		return def
	}
	return value == "true" || value == "1" || value == "yes"
}

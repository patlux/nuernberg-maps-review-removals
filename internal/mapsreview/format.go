package mapsreview

import (
	"math"
	"strconv"
	"strings"
)

func Round(value float64, digits int) float64 {
	factor := math.Pow10(digits)
	return math.Round(value*factor) / factor
}

func FormatGermanFloat(value float64, digits int) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "–"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	formatted := strconv.FormatFloat(value, 'f', digits, 64)
	parts := strings.SplitN(formatted, ".", 2)
	whole := addGermanThousands(parts[0])
	if negative {
		whole = "-" + whole
	}
	if digits == 0 {
		return whole
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	for len(frac) < digits {
		frac += "0"
	}
	return whole + "," + frac
}

func FormatGermanInt(value int) string {
	return addGermanThousands(strconv.Itoa(value))
}

func addGermanThousands(s string) string {
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	prefix := len(s) % 3
	if prefix == 0 {
		prefix = 3
	}
	b.WriteString(s[:prefix])
	for i := prefix; i < len(s); i += 3 {
		b.WriteByte('.')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func FormatPtrFloat(value *float64, digits int) string {
	if value == nil {
		return "–"
	}
	return FormatGermanFloat(*value, digits)
}

func FormatPtrInt(value *int) string {
	if value == nil {
		return "–"
	}
	return FormatGermanInt(*value)
}

package mapsreview

import "sort"

func ComputeMetrics(row *Place) {
	row.DeletionRatioPct = nil
	row.RealRatingAdjusted = nil
	if row.RemovedEstimate == nil || row.ReviewCount == nil || row.Rating == nil || *row.ReviewCount <= 0 {
		return
	}
	total := float64(*row.ReviewCount) + *row.RemovedEstimate
	if total <= 0 {
		return
	}
	deletionRatio := Round((*row.RemovedEstimate/total)*100, 2)
	realRating := Round(((*row.Rating*float64(*row.ReviewCount))+*row.RemovedEstimate)/total, 3)
	row.DeletionRatioPct = FloatPtr(deletionRatio)
	row.RealRatingAdjusted = FloatPtr(realRating)
}

func ValidRows(rows []Place) []Place {
	out := make([]Place, 0, len(rows))
	for _, row := range rows {
		if row.Status == "success" && row.Name != "" && row.ReviewCount != nil && row.Rating != nil {
			out = append(out, row)
		}
	}
	return out
}

func RemovedRows(rows []Place) []Place {
	out := make([]Place, 0, len(rows))
	for _, row := range ValidRows(rows) {
		if row.HasDefamationNotice {
			out = append(out, row)
		}
	}
	return out
}

func RemovedRange(row Place) string {
	if !row.HasDefamationNotice || row.RemovedMin == nil {
		return ""
	}
	if row.RemovedMax == nil {
		return "über " + FormatGermanInt(*row.RemovedMin)
	}
	if *row.RemovedMin == *row.RemovedMax {
		return FormatGermanInt(*row.RemovedMin)
	}
	return FormatGermanInt(*row.RemovedMin) + "–" + FormatGermanInt(*row.RemovedMax)
}

func RemovedSortValue(row Place) float64 {
	if row.RemovedEstimate != nil {
		return *row.RemovedEstimate
	}
	if row.RemovedMax == nil {
		return float64(IntValue(row.RemovedMin) + 50)
	}
	return (float64(IntValue(row.RemovedMin)) + float64(*row.RemovedMax)) / 2
}

type PostcodeGroup struct {
	Postcode string
	Rows     []Place
}

func GroupByPostcode(rows []Place) []PostcodeGroup {
	groups := map[string][]Place{}
	for _, row := range rows {
		postcode := StringValue(row.Postcode)
		if postcode == "" {
			postcode = "unknown"
		}
		groups[postcode] = append(groups[postcode], row)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]PostcodeGroup, 0, len(keys))
	for _, key := range keys {
		out = append(out, PostcodeGroup{Postcode: key, Rows: groups[key]})
	}
	return out
}

func BinFor(row Place) string {
	if !row.HasDefamationNotice {
		return "Keine Löschung"
	}
	min := IntValue(row.RemovedMin)
	max := min
	if row.RemovedMax != nil {
		max = *row.RemovedMax
	}
	switch {
	case min == 1 && max == 1:
		return "Eine gelöscht"
	case min <= 2 && max <= 5:
		return "2–5 gelöscht"
	case min <= 6 && max <= 10:
		return "6–10 gelöscht"
	case min <= 11 && max <= 20:
		return "11–20 gelöscht"
	case min <= 21 && max <= 50:
		return "21–50 gelöscht"
	case min <= 51 && max <= 100:
		return "51–100 gelöscht"
	case min <= 101 && max <= 200:
		return "101–200 gelöscht"
	case min <= 201 && max <= 250:
		return "201–250 gelöscht"
	default:
		return "Über 250 gelöscht"
	}
}

var BinOrder = []string{
	"Keine Löschung", "Eine gelöscht", "2–5 gelöscht", "6–10 gelöscht",
	"11–20 gelöscht", "21–50 gelöscht", "51–100 gelöscht", "101–200 gelöscht",
	"201–250 gelöscht", "Über 250 gelöscht",
}

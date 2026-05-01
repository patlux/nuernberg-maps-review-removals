package mapsreview

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func EnsureDirForPath(fileOrDir string) error {
	dir := fileOrDir
	if filepath.Ext(fileOrDir) != "" {
		dir = filepath.Dir(fileOrDir)
	}
	return os.MkdirAll(dir, 0o755)
}

func ReadJSON[T any](file string, fallback T) (T, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fallback, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fallback, err
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return fallback, err
	}
	return value, nil
}

func WriteJSON(file string, value any) error {
	if err := EnsureDirForPath(file); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(file, data, 0o644)
}

func SortPlaces(rows []Place) {
	sort.SliceStable(rows, func(i, j int) bool {
		pi := StringValue(rows[i].Postcode)
		pj := StringValue(rows[j].Postcode)
		if pi != pj {
			return pi < pj
		}
		return rows[i].Name < rows[j].Name
	})
}

func WritePlacesCSV(file string, rows []Place) error {
	if err := EnsureDirForPath(file); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	columns := []string{
		"id", "name", "postcode", "address", "rating", "reviewCount", "category", "lat", "lng", "bezirkId", "bezirkName",
		"hasDefamationNotice", "removedMin", "removedMax", "removedEstimate",
		"deletionRatioPct", "realRatingAdjusted", "removedText", "url", "readAt",
		"placeState", "status", "error",
	}
	if err := w.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write([]string{
			row.ID,
			row.Name,
			StringValue(row.Postcode),
			StringValue(row.Address),
			floatCSV(row.Rating),
			intCSV(row.ReviewCount),
			StringValue(row.Category),
			floatCSV(row.Lat),
			floatCSV(row.Lng),
			StringValue(row.BezirkID),
			StringValue(row.BezirkName),
			strconv.FormatBool(row.HasDefamationNotice),
			intCSV(row.RemovedMin),
			intCSV(row.RemovedMax),
			floatCSV(row.RemovedEstimate),
			floatCSV(row.DeletionRatioPct),
			floatCSV(row.RealRatingAdjusted),
			StringValue(row.RemovedText),
			row.URL,
			row.ReadAt,
			row.PlaceState,
			row.Status,
			StringValue(row.Error),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func intCSV(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func floatCSV(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func MustWriteJSON(file string, value any) {
	if err := WriteJSON(file, value); err != nil {
		panic(fmt.Sprintf("write %s: %v", file, err))
	}
}

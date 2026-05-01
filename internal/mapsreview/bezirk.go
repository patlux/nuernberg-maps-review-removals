package mapsreview

import (
	"embed"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Source: Stadt Nürnberg Bezirksatlas InstantAtlas layer
// https://online-service2.nuernberg.de/geoinf/ia_bezirksatlas/
//
//go:embed data/nuernberg_statistische_bezirke.json
var bezirkFS embed.FS

type Bezirk struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BezirkBoundary struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Label    string        `json:"label"`
	Polygons [][][]float64 `json:"polygons"`
}

type bezirkMapSource struct {
	PixelWidth  float64               `json:"pixelWidth"`
	PixelHeight float64               `json:"pixelHeight"`
	BoundingBox string                `json:"boundingBox"`
	Features    []bezirkFeatureSource `json:"features"`
}

type bezirkFeatureSource struct {
	ID    string      `json:"d"`
	Name  string      `json:"n"`
	Paths [][]float64 `json:"p"`
}

type bezirkPolygon struct {
	Bezirk
	Rings [][]point
	MinX  float64
	MinY  float64
	MaxX  float64
	MaxY  float64
}

type bezirkIndex struct {
	MinX        float64
	MinY        float64
	MaxX        float64
	MaxY        float64
	PixelWidth  float64
	PixelHeight float64
	Polygons    []bezirkPolygon
}

type point struct {
	X float64
	Y float64
}

var (
	bezirkOnce  sync.Once
	bezirke     *bezirkIndex
	bezirkError error
)

func AssignBezirk(lat, lng float64) *Bezirk {
	return assignBezirk(lat, lng, "", false)
}

func AssignBezirkForPostcode(lat, lng float64, postcode string) *Bezirk {
	return assignBezirk(lat, lng, postcode, true)
}

func AllBezirke() []Bezirk {
	idx, err := loadBezirkIndex()
	if err != nil {
		return nil
	}
	out := make([]Bezirk, 0, len(idx.Polygons))
	for _, polygon := range idx.Polygons {
		out = append(out, polygon.Bezirk)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func BezirkBoundaries() []BezirkBoundary {
	idx, err := loadBezirkIndex()
	if err != nil {
		return nil
	}
	out := make([]BezirkBoundary, 0, len(idx.Polygons))
	for _, polygon := range idx.Polygons {
		boundary := BezirkBoundary{
			ID:    polygon.ID,
			Name:  polygon.Name,
			Label: polygon.ID + " " + polygon.Name,
		}
		for _, ring := range polygon.Rings {
			points := make([][]float64, 0, len(ring)+1)
			for _, p := range ring {
				lat, lng := idx.unproject(p)
				points = append(points, []float64{roundCoord(lat), roundCoord(lng)})
			}
			if len(points) > 0 {
				points = append(points, points[0])
			}
			boundary.Polygons = append(boundary.Polygons, points)
		}
		out = append(out, boundary)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func assignBezirk(lat, lng float64, postcode string, allowFallback bool) *Bezirk {
	idx, err := loadBezirkIndex()
	if err != nil || !validCoordinate(lat, lng) {
		return nil
	}
	p := idx.project(lat, lng)
	for _, polygon := range idx.Polygons {
		if polygon.contains(p) {
			bezirk := polygon.Bezirk
			return &bezirk
		}
	}
	if allowFallback && NurembergPostcodeSet[strings.TrimSpace(postcode)] {
		if polygon := idx.nearest(p); polygon != nil {
			bezirk := polygon.Bezirk
			return &bezirk
		}
	}
	return nil
}

func loadBezirkIndex() (*bezirkIndex, error) {
	bezirkOnce.Do(func() {
		data, err := bezirkFS.ReadFile("data/nuernberg_statistische_bezirke.json")
		if err != nil {
			bezirkError = err
			return
		}
		var source bezirkMapSource
		if err := json.Unmarshal(data, &source); err != nil {
			bezirkError = err
			return
		}
		bbox := parseBoundingBox(source.BoundingBox)
		idx := &bezirkIndex{
			MinX: bbox[0], MinY: bbox[1], MaxX: bbox[2], MaxY: bbox[3],
			PixelWidth: source.PixelWidth, PixelHeight: source.PixelHeight,
		}
		for _, feature := range source.Features {
			polygon := bezirkPolygon{Bezirk: Bezirk{ID: feature.ID, Name: feature.Name}}
			polygon.MinX, polygon.MinY = math.Inf(1), math.Inf(1)
			polygon.MaxX, polygon.MaxY = math.Inf(-1), math.Inf(-1)
			for _, path := range feature.Paths {
				ring := decodeBezirkPath(path)
				if len(ring) < 3 {
					continue
				}
				polygon.Rings = append(polygon.Rings, ring)
				for _, p := range ring {
					polygon.MinX = math.Min(polygon.MinX, p.X)
					polygon.MinY = math.Min(polygon.MinY, p.Y)
					polygon.MaxX = math.Max(polygon.MaxX, p.X)
					polygon.MaxY = math.Max(polygon.MaxY, p.Y)
				}
			}
			if len(polygon.Rings) == 0 {
				continue
			}
			idx.Polygons = append(idx.Polygons, polygon)
		}
		bezirke = idx
	})
	return bezirke, bezirkError
}

func parseBoundingBox(value string) [4]float64 {
	parts := strings.Fields(value)
	var out [4]float64
	for i := range out {
		if i < len(parts) {
			out[i] = parseFloat(parts[i])
		}
	}
	return out
}

func parseFloat(value string) float64 {
	out, _ := strconv.ParseFloat(value, 64)
	return out
}

func decodeBezirkPath(path []float64) []point {
	if len(path) < 4 {
		return nil
	}
	x := path[0]
	y := path[1]
	ring := []point{{X: x, Y: y}}
	for i := 2; i+1 < len(path); i += 2 {
		x += path[i]
		y += path[i+1]
		ring = append(ring, point{X: x, Y: y})
	}
	return ring
}

func (idx *bezirkIndex) project(lat, lng float64) point {
	const earthRadius = 6378137.0
	mercatorX := earthRadius * lng * math.Pi / 180
	mercatorY := earthRadius * math.Log(math.Tan(math.Pi/4+(lat*math.Pi/180)/2))
	return point{
		X: ((mercatorX - idx.MinX) / (idx.MaxX - idx.MinX)) * idx.PixelWidth,
		Y: ((mercatorY - idx.MinY) / (idx.MaxY - idx.MinY)) * idx.PixelHeight,
	}
}

func (idx *bezirkIndex) unproject(p point) (float64, float64) {
	const earthRadius = 6378137.0
	mercatorX := idx.MinX + (p.X/idx.PixelWidth)*(idx.MaxX-idx.MinX)
	mercatorY := idx.MinY + (p.Y/idx.PixelHeight)*(idx.MaxY-idx.MinY)
	lng := mercatorX / earthRadius * 180 / math.Pi
	lat := (2*math.Atan(math.Exp(mercatorY/earthRadius)) - math.Pi/2) * 180 / math.Pi
	return lat, lng
}

func roundCoord(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func (polygon bezirkPolygon) contains(p point) bool {
	if p.X < polygon.MinX || p.X > polygon.MaxX || p.Y < polygon.MinY || p.Y > polygon.MaxY {
		return false
	}
	inside := false
	for _, ring := range polygon.Rings {
		if pointInRing(p, ring) {
			inside = !inside
		}
	}
	return inside
}

func (idx *bezirkIndex) nearest(p point) *bezirkPolygon {
	var best *bezirkPolygon
	bestDistance := math.Inf(1)
	for i := range idx.Polygons {
		polygon := &idx.Polygons[i]
		distance := polygon.distanceSquared(p)
		if distance < bestDistance {
			bestDistance = distance
			best = polygon
		}
	}
	return best
}

func (polygon bezirkPolygon) distanceSquared(p point) float64 {
	best := math.Inf(1)
	for _, ring := range polygon.Rings {
		j := len(ring) - 1
		for i := range ring {
			distance := pointSegmentDistanceSquared(p, ring[j], ring[i])
			if distance < best {
				best = distance
			}
			j = i
		}
	}
	return best
}

func pointSegmentDistanceSquared(p, a, b point) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	if dx == 0 && dy == 0 {
		px := p.X - a.X
		py := p.Y - a.Y
		return px*px + py*py
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	closest := point{X: a.X + t*dx, Y: a.Y + t*dy}
	px := p.X - closest.X
	py := p.Y - closest.Y
	return px*px + py*py
}

// pointInRing uses the even-odd ray-casting algorithm.
// Points exactly on horizontal edges (p.Y == yi == yj) are not handled;
// in practice this is fine for real bezirk boundary data in Mercator projection.
func pointInRing(p point, ring []point) bool {
	inside := false
	j := len(ring) - 1
	for i := range ring {
		xi, yi := ring[i].X, ring[i].Y
		xj, yj := ring[j].X, ring[j].Y
		intersects := (yi > p.Y) != (yj > p.Y)
		if intersects {
			x := (xj-xi)*(p.Y-yi)/(yj-yi) + xi
			if p.X < x {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

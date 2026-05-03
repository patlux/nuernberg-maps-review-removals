package mapsreview

const (
	OutputDir     = "output"
	ResultsJSON   = "output/places.json"
	ResultsCSV    = "output/places.csv"
	DiscoveryJSON = "output/discovery.json"
	MetadataJSON  = "output/metadata.json"
)

var DefaultCity = "Nürnberg"

var NurembergPostcodes = []string{
	"90402", "90403", "90408", "90409", "90411", "90419", "90425", "90427",
	"90429", "90431", "90439", "90441", "90443", "90449", "90451", "90453",
	"90455", "90459", "90461", "90469", "90471", "90473", "90475", "90478",
	"90480", "90482", "90489", "90491",
}

var DefaultQueries = []string{
	"restaurant", "café", "imbiss", "pizzeria", "bäckerei",
	"döner", "burger", "sushi", "schnitzel", "frühstück", "brunch",
}

var NurembergPostcodeSet = func() map[string]bool {
	set := make(map[string]bool, len(NurembergPostcodes))
	for _, postcode := range NurembergPostcodes {
		set[postcode] = true
	}
	return set
}()

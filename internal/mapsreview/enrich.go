package mapsreview

func EnrichPlaceLocation(row *Place) {
	if row == nil {
		return
	}
	if row.Lat == nil || row.Lng == nil {
		if coords := ExtractCoordinates(row.URL); coords != nil {
			row.Lat = FloatPtr(coords.Lat)
			row.Lng = FloatPtr(coords.Lng)
		}
	}
	row.BezirkID = nil
	row.BezirkName = nil
	if row.Lat == nil || row.Lng == nil {
		return
	}
	if bezirk := AssignBezirkForPostcode(*row.Lat, *row.Lng, StringValue(row.Postcode)); bezirk != nil {
		row.BezirkID = StringPtr(bezirk.ID)
		row.BezirkName = StringPtr(bezirk.Name)
	}
}

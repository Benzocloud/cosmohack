package source

import (
	"fmt"
	"math"
)

// SatelliteIndices contains sensor indices for one satellite observation.
// Nil values mean that the provider had no usable pixels for that index.
type SatelliteIndices struct {
	S2NDVI *float64
	S2EVI  *float64
	S2NDWI *float64
}

func (i SatelliteIndices) Primary() *float64 {
	if i.S2NDVI == nil {
		return nil
	}
	v := *i.S2NDVI
	return &v
}

func (i SatelliteIndices) Values() SatelliteIndices {
	return SatelliteIndices{
		S2NDVI: copyFloat(i.S2NDVI),
		S2EVI:  copyFloat(i.S2EVI),
		S2NDWI: copyFloat(i.S2NDWI),
	}
}

func validateSatelliteIndices(indices *SatelliteIndices) error {
	if indices == nil {
		return nil
	}
	for name, value := range map[string]*float64{
		"s2_ndvi": indices.S2NDVI,
		"s2_evi":  indices.S2EVI,
		"s2_ndwi": indices.S2NDWI,
	} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return fmt.Errorf("%s is not finite", name)
		}
	}
	return nil
}

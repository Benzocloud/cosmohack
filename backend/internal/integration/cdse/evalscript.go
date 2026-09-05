package cdse

import (
	"fmt"
	"strings"
)

var maskedSceneClasses = []int{0, 1, 3, 8, 9, 10, 11}

const evalscriptTemplate = `//VERSION=3
function setup() {
  return {
    input: [{bands: ["B02", "B04", "B08", "B11", "SCL", "dataMask"]}],
    output: [
      {id: "ndvi", bands: 1, sampleType: "FLOAT32"},
      {id: "evi", bands: 1, sampleType: "FLOAT32"},
      {id: "ndwi", bands: 1, sampleType: "FLOAT32"},
      {id: "dataMask", bands: 1}
    ]
  };
}

function evaluatePixel(sample) {
  // NDWI по определению Gao: ближний ИК (B08) и SWIR (B11).
  var masked = [%s];
  var ndviDenominator = sample.B08 + sample.B04;
  var eviDenominator = sample.B08 + 6 * sample.B04 - 7.5 * sample.B02 + 1;
  var ndwiDenominator = sample.B08 + sample.B11;
  var usable = sample.dataMask === 1 && masked.indexOf(sample.SCL) < 0 &&
    ndviDenominator > 0 && eviDenominator !== 0 && ndwiDenominator !== 0;
  var ndvi = usable ? (sample.B08 - sample.B04) / (sample.B08 + sample.B04) : 0;
  var evi = usable ? 2.5 * (sample.B08 - sample.B04) / eviDenominator : 0;
  var ndwi = usable ? (sample.B08 - sample.B11) / ndwiDenominator : 0;
  return {ndvi: [ndvi], evi: [evi], ndwi: [ndwi], dataMask: [usable ? 1 : 0]};
}
`

func evalscript() string {
	return fmt.Sprintf(evalscriptTemplate, joinClasses(maskedSceneClasses))
}

func joinClasses(classes []int) string {
	parts := make([]string, 0, len(classes))
	for _, class := range classes {
		parts = append(parts, fmt.Sprint(class))
	}
	return strings.Join(parts, ", ")
}

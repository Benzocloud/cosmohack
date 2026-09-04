package cdse

import (
	"fmt"
	"strings"
)

var maskedSceneClasses = []int{0, 1, 3, 8, 9, 10, 11}

const evalscriptTemplate = `//VERSION=3
function setup() {
  return {
    input: [{bands: ["B04", "B08", "SCL", "dataMask"]}],
    output: [
      {id: "ndvi", bands: 1, sampleType: "FLOAT32"},
      {id: "dataMask", bands: 1}
    ]
  };
}

function evaluatePixel(sample) {
  var masked = [%s];
  var usable = sample.dataMask === 1 && masked.indexOf(sample.SCL) < 0 && (sample.B08 + sample.B04) > 0;
  var ndvi = usable ? (sample.B08 - sample.B04) / (sample.B08 + sample.B04) : 0;
  return {ndvi: [ndvi], dataMask: [usable ? 1 : 0]};
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

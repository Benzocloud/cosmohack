package handler

import (
	"math"
	"strconv"
	"strings"
)

func parseBBox(s string) (bbox, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return bbox{}, errInvalidBBox
	}
	var nums [4]float64
	for i, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return bbox{}, errInvalidBBox
		}
		nums[i] = v
	}
	b := bbox{MinLon: nums[0], MinLat: nums[1], MaxLon: nums[2], MaxLat: nums[3]}
	if b.MinLon < -180 || b.MaxLon > 180 || b.MinLat < -90 || b.MaxLat > 90 {
		return bbox{}, errInvalidBBox
	}
	if b.MinLon >= b.MaxLon || b.MinLat >= b.MaxLat {
		return bbox{}, errInvalidBBox
	}
	return b, nil
}

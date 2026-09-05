package cdse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

type bandStats struct {
	Min         statisticNumber `json:"min"`
	Max         statisticNumber `json:"max"`
	Mean        statisticNumber `json:"mean"`
	StDev       statisticNumber `json:"stDev"`
	SampleCount int             `json:"sampleCount"`
	NoDataCount int             `json:"noDataCount"`
}

type statisticNumber struct {
	value *float64
}

func (n *statisticNumber) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		n.value = nil
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		data = []byte(text)
	}
	text := string(data)
	if strings.EqualFold(text, "nan") || strings.EqualFold(text, "infinity") ||
		strings.EqualFold(text, "+infinity") || strings.EqualFold(text, "-infinity") {
		n.value = nil
		return nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		n.value = nil
		return nil
	}
	n.value = &value
	return nil
}

type bandOutput struct {
	Bands map[string]struct {
		Stats bandStats `json:"stats"`
	} `json:"bands"`
}

type intervalBounds struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type statisticsItem struct {
	Interval intervalBounds        `json:"interval"`
	Outputs  map[string]bandOutput `json:"outputs"`
	Error    *struct {
		Type string `json:"type"`
	} `json:"error"`
}

type statisticsResponse struct {
	Status string           `json:"status"`
	Data   []statisticsItem `json:"data"`
}

func (r *statisticsResponse) samples(minValidFraction float64) ([]source.SatelliteSample, []string, error) {
	samples := make([]source.SatelliteSample, 0, len(r.Data))
	notes := make([]string, 0, 2)
	failed := 0
	for _, item := range r.Data {
		interval, err := item.dateRange()
		if err != nil {
			return nil, nil, err
		}
		if item.Error != nil {
			failed++
			sample, buildErr := newSample(interval, nil, nil, false, source.ReasonProviderEntryFailed)
			if buildErr != nil {
				return nil, nil, buildErr
			}
			samples = append(samples, sample)
			continue
		}
		sample, err := item.sample(interval, minValidFraction)
		if err != nil {
			return nil, nil, err
		}
		samples = append(samples, sample)
	}
	if failed > 0 {
		notes = append(notes, fmt.Sprintf("Интервалов с ошибкой расчёта у провайдера: %d", failed))
	}
	return samples, notes, nil
}

func (i statisticsItem) dateRange() (source.DateRange, error) {
	from, err := parseInterval(i.Interval.From)
	if err != nil {
		return source.DateRange{}, err
	}
	to, err := parseInterval(i.Interval.To)
	if err != nil {
		return source.DateRange{}, err
	}
	last := to.AddDays(-1)
	if last.Before(from) {
		last = from
	}
	interval, buildErr := source.NewDateRange(from, last)
	if buildErr != nil {
		return source.DateRange{}, domain.WrapProviderError(domain.FailureMalformed, ProviderName, buildErr,
			"aggregation interval is invalid")
	}
	return interval, nil
}

func (i statisticsItem) sample(interval source.DateRange, minValidFraction float64) (source.SatelliteSample, error) {
	stats, found := i.stats()
	if !found {
		return newSample(interval, nil, nil, false, source.ReasonProviderInterval)
	}
	if stats.SampleCount <= 0 || stats.SampleCount <= stats.NoDataCount {
		return newSample(interval, nil, nil, false, source.ReasonNoValidSamples)
	}
	fraction := float64(stats.SampleCount-stats.NoDataCount) / float64(stats.SampleCount)
	validFraction := source.Float(math.Min(math.Max(fraction, 0), 1))
	if stats.Mean.value == nil {
		return newSample(interval, nil, validFraction, false, source.ReasonNoValidSamples)
	}
	value := source.Float(*stats.Mean.value)
	switch {
	case *value < -1 || *value > 1:
		return newSample(interval, value, validFraction, false, reasonOutOfRange)
	case *validFraction < minValidFraction:
		return newSample(interval, value, validFraction, false, source.ReasonLowValidFraction)
	default:
		return newSample(interval, value, validFraction, true, "")
	}
}

func (i statisticsItem) stats() (bandStats, bool) {
	output, found := i.Outputs["ndvi"]
	if !found {
		return bandStats{}, false
	}
	if band, found := output.Bands["B0"]; found {
		return band.Stats, true
	}
	for _, band := range output.Bands {
		return band.Stats, true
	}
	return bandStats{}, false
}

func newSample(interval source.DateRange, value, validFraction *float64, usable bool, reason string) (source.SatelliteSample, error) {
	sample, err := source.NewSatelliteSample(interval, value, validFraction, usable, reason)
	if err != nil {
		return source.SatelliteSample{}, domain.WrapProviderError(domain.FailureMalformed, ProviderName, err,
			"observation for interval %s was rejected", interval)
	}
	return sample, nil
}

func parseInterval(value string) (source.Date, error) {
	if value == "" {
		return source.Date{}, domain.NewProviderError(domain.FailureMalformed, ProviderName,
			"aggregation interval contains no dates")
	}
	if moment, err := time.Parse(time.RFC3339, value); err == nil {
		return source.DateFromTime(moment), nil
	}
	date, err := source.ParseDate(value)
	if err != nil {
		return source.Date{}, domain.WrapProviderError(domain.FailureMalformed, ProviderName, err,
			"aggregation interval date %q could not be parsed", value)
	}
	return date, nil
}

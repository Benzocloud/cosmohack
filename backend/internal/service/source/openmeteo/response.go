package openmeteo

import (
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

type responseDocument struct {
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	Elevation        float64 `json:"elevation"`
	UTCOffsetSeconds int     `json:"utc_offset_seconds"`
	Timezone         string  `json:"timezone"`
	DailyUnits       struct {
		Time          string `json:"time"`
		Temperature   string `json:"temperature_2m_mean"`
		Precipitation string `json:"precipitation_sum"`
	} `json:"daily_units"`
	Daily struct {
		Time          []string   `json:"time"`
		Temperature   []*float64 `json:"temperature_2m_mean"`
		Precipitation []*float64 `json:"precipitation_sum"`
	} `json:"daily"`
}

func (d *responseDocument) validate() error {
	if d.UTCOffsetSeconds != expectedOffsetSeconds {
		return source.NewProviderError(source.FailureMalformed, ProviderName,
			"сдвиг времени %d секунд вместо UTC", d.UTCOffsetSeconds)
	}
	if d.DailyUnits.Temperature != expectedTemperatureUnit {
		return source.NewProviderError(source.FailureMalformed, ProviderName,
			"единица температуры %q вместо %q", d.DailyUnits.Temperature, expectedTemperatureUnit)
	}
	if d.DailyUnits.Precipitation != expectedPrecipitationUnit {
		return source.NewProviderError(source.FailureMalformed, ProviderName,
			"единица осадков %q вместо %q", d.DailyUnits.Precipitation, expectedPrecipitationUnit)
	}
	if len(d.Daily.Time) != len(d.Daily.Temperature) || len(d.Daily.Time) != len(d.Daily.Precipitation) {
		return source.NewProviderError(source.FailureMalformed, ProviderName,
			"длины массивов дат и значений не совпадают")
	}
	return nil
}

func (d *responseDocument) days(period source.DateRange) ([]source.WeatherDay, []string, error) {
	days := make([]source.WeatherDay, 0, len(d.Daily.Time))
	notes := make([]string, 0, 2)
	outside, negative := 0, 0
	for index, text := range d.Daily.Time {
		date, err := source.ParseDate(text)
		if err != nil {
			return nil, nil, source.WrapProviderError(source.FailureMalformed, ProviderName, err,
				"дата %q не разбирается", text)
		}
		if !period.Contains(date) {
			outside++
			continue
		}
		precipitation := d.Daily.Precipitation[index]
		if precipitation != nil && *precipitation < 0 {
			negative++
			precipitation = nil
		}
		day, err := source.NewWeatherDay(date, d.Daily.Temperature[index], precipitation)
		if err != nil {
			return nil, nil, source.WrapProviderError(source.FailureMalformed, ProviderName, err,
				"погодные значения на %s не приняты", date)
		}
		days = append(days, day)
	}
	if outside > 0 {
		notes = append(notes, fmt.Sprintf("Погодных дней вне периода анализа: %d", outside))
	}
	if negative > 0 {
		notes = append(notes, fmt.Sprintf("Отрицательные суммы осадков заменены на отсутствующие значения: %d", negative))
	}
	return days, notes, nil
}

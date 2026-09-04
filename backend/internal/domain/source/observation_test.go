package source_test

import (
	"math"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

func mustDate(t *testing.T, text string) source.Date {
	t.Helper()
	date, err := source.ParseDate(text)
	if err != nil {
		t.Fatalf("date %s was not parsed: %v", text, err)
	}
	return date
}

func mustRange(t *testing.T, from, to string) source.DateRange {
	t.Helper()
	period, err := source.ParseDateRange(from, to)
	if err != nil {
		t.Fatalf("period %s..%s was not built: %v", from, to, err)
	}
	return period
}

func TestObservationBuilderAcceptsMeasurement(t *testing.T) {
	interval := mustRange(t, "2025-06-01", "2025-06-05")
	observation, err := source.NewObservationBuilder(mustDate(t, "2025-06-03")).
		Measured(0.72, "satellite-1", interval, source.Float(0.9)).
		Build()
	if err != nil {
		t.Fatalf("наблюдение не построено: %v", err)
	}
	if observation.Quality() != source.QualityUsable || *observation.PrimaryNDVI() != 0.72 {
		t.Fatalf("качество %s, значение %v", observation.Quality(), observation.PrimaryNDVI())
	}
	if observation.MissingReason() != "" {
		t.Fatalf("пригодное наблюдение содержит причину %q", observation.MissingReason())
	}
}

func TestObservationBuilderRejectsInconsistentStates(t *testing.T) {
	interval := mustRange(t, "2025-06-01", "2025-06-05")
	cases := map[string]func() (source.Observation, error){
		"дата вне интервала": func() (source.Observation, error) {
			return source.NewObservationBuilder(mustDate(t, "2025-06-09")).
				Measured(0.5, "satellite-1", interval, nil).Build()
		},
		"пригодное без источника": func() (source.Observation, error) {
			return source.NewObservationBuilder(mustDate(t, "2025-06-03")).
				Measured(0.5, "", interval, nil).Build()
		},
		"отбраковка без причины": func() (source.Observation, error) {
			return source.NewObservationBuilder(mustDate(t, "2025-06-03")).
				Rejected(source.Float(0.5), "satellite-1", interval, "", nil).Build()
		},
		"пропуск без причины": func() (source.Observation, error) {
			return source.NewObservationBuilder(mustDate(t, "2025-06-03")).Missing("").Build()
		},
		"доля вне диапазона": func() (source.Observation, error) {
			return source.NewObservationBuilder(mustDate(t, "2025-06-03")).
				Measured(0.5, "satellite-1", interval, source.Float(1.5)).Build()
		},
		"неконечное значение": func() (source.Observation, error) {
			return source.NewObservationBuilder(mustDate(t, "2025-06-03")).
				Measured(math.Inf(1), "satellite-1", interval, nil).Build()
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); err == nil {
				t.Fatal("некорректное наблюдение принято")
			}
		})
	}
}

func TestObservationBuilderKeepsRejectedValue(t *testing.T) {
	interval := mustRange(t, "2025-06-01", "2025-06-05")
	observation, err := source.NewObservationBuilder(mustDate(t, "2025-06-03")).
		Rejected(source.Float(0.31), "satellite-1", interval, source.ReasonLowValidFraction, source.Float(0.2)).
		Build()
	if err != nil {
		t.Fatalf("наблюдение не построено: %v", err)
	}
	if observation.Quality() != source.QualityUnusable {
		t.Fatalf("качество %s", observation.Quality())
	}
	if observation.PrimaryNDVI() == nil || *observation.PrimaryNDVI() != 0.31 {
		t.Fatal("исходное значение отбракованного наблюдения потеряно")
	}
	if observation.MissingReason() != source.ReasonLowValidFraction {
		t.Fatalf("причина %q", observation.MissingReason())
	}
}

func TestWeatherRejectsNegativePrecipitation(t *testing.T) {
	if _, err := source.NewWeather("weather-1", source.Float(20), source.Float(-1)); err == nil {
		t.Fatal("отрицательные осадки приняты")
	}
	if _, err := source.NewWeather("", source.Float(20), source.Float(1)); err == nil {
		t.Fatal("погода без источника принята")
	}
}

func TestReferenceRequiresExcludedTargetYear(t *testing.T) {
	if _, err := source.NewReference("reference-1", 0.5, 0.1, 5, false); err == nil {
		t.Fatal("фон без исключения целевого года принят")
	}
	if _, err := source.NewReference("reference-1", 0.5, -0.1, 5, true); err == nil {
		t.Fatal("отрицательное стандартное отклонение принято")
	}
	if _, err := source.NewReference("reference-1", 0.5, 0.1, 0, true); err == nil {
		t.Fatal("нулевое число лет принято")
	}
}

func TestObservationAccessorsReturnCopies(t *testing.T) {
	interval := mustRange(t, "2025-06-01", "2025-06-05")
	observation, err := source.NewObservationBuilder(mustDate(t, "2025-06-03")).
		Measured(0.6, "satellite-1", interval, source.Float(0.8)).Build()
	if err != nil {
		t.Fatalf("наблюдение не построено: %v", err)
	}
	value := observation.PrimaryNDVI()
	*value = 0.1
	if *observation.PrimaryNDVI() != 0.6 {
		t.Fatal("значение наблюдения изменено через возвращённый указатель")
	}
}

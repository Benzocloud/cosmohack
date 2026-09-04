package source_test

import (
	"encoding/json"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

func mustDate(t *testing.T, text string) source.Date {
	t.Helper()
	date, err := source.ParseDate(text)
	if err != nil {
		t.Fatalf("дата %s не разобрана: %v", text, err)
	}
	return date
}

func mustRange(t *testing.T, from, to string) source.DateRange {
	t.Helper()
	period, err := source.ParseDateRange(from, to)
	if err != nil {
		t.Fatalf("период %s..%s не построен: %v", from, to, err)
	}
	return period
}

func TestParseDateRejectsWrongFormat(t *testing.T) {
	for _, text := range []string{"", "01.06.2025", "2025-6-1", "2025-13-01", "2025-06-01T00:00:00Z"} {
		if _, err := source.ParseDate(text); err == nil {
			t.Fatalf("значение %q принято как дата", text)
		}
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	payload, err := json.Marshal(mustDate(t, "2025-06-01"))
	if err != nil {
		t.Fatalf("дата не сериализована: %v", err)
	}
	if string(payload) != `"2025-06-01"` {
		t.Fatalf("сериализация %s", payload)
	}
	var restored source.Date
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatalf("дата не разобрана: %v", err)
	}
	if !restored.Equal(mustDate(t, "2025-06-01")) {
		t.Fatalf("после разбора получено %s", restored)
	}
}

func TestDateRangeDays(t *testing.T) {
	period := mustRange(t, "2025-06-01", "2025-06-10")
	if period.Days() != 10 {
		t.Fatalf("дней %d, ожидалось 10", period.Days())
	}
	dates := period.Dates()
	if len(dates) != 10 || dates[0].String() != "2025-06-01" || dates[9].String() != "2025-06-10" {
		t.Fatalf("границы периода %v", dates)
	}
	if !period.Contains(mustDate(t, "2025-06-05")) || period.Contains(mustDate(t, "2025-06-11")) {
		t.Fatal("проверка вхождения даты в период неверна")
	}
}

func TestDateRangeRejectsReversedBounds(t *testing.T) {
	if _, err := source.ParseDateRange("2025-06-10", "2025-06-01"); err == nil {
		t.Fatal("период с началом позже конца принят")
	}
}

func TestDateRangeSpansLeapDay(t *testing.T) {
	period := mustRange(t, "2024-02-28", "2024-03-01")
	if period.Days() != 3 {
		t.Fatalf("дней %d, ожидалось 3 с учётом 29 февраля", period.Days())
	}
}

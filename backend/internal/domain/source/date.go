package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

var errEmptyDate = errors.New("date is required")

type Date struct {
	value time.Time
}

func NewDate(year int, month time.Month, day int) Date {
	return Date{value: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

func ParseDate(text string) (Date, error) {
	if text == "" {
		return Date{}, errEmptyDate
	}
	parsed, err := time.ParseInLocation(DateLayout, text, time.UTC)
	if err != nil {
		return Date{}, fmt.Errorf("date %q does not match format YYYY-MM-DD", text)
	}
	return Date{value: parsed}, nil
}

func DateFromTime(moment time.Time) Date {
	utc := moment.UTC()
	return NewDate(utc.Year(), utc.Month(), utc.Day())
}

func (d Date) IsZero() bool {
	return d.value.IsZero()
}

func (d Date) Time() time.Time {
	return d.value
}

func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.value.Format(DateLayout)
}

func (d Date) AddDays(days int) Date {
	return Date{value: d.value.AddDate(0, 0, days)}
}

func (d Date) Compare(other Date) int {
	return d.value.Compare(other.value)
}

func (d Date) Before(other Date) bool {
	return d.Compare(other) < 0
}

func (d Date) After(other Date) bool {
	return d.Compare(other) > 0
}

func (d Date) Equal(other Date) bool {
	return d.Compare(other) == 0
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return nil, errEmptyDate
	}
	return json.Marshal(d.String())
}

func (d *Date) UnmarshalJSON(payload []byte) error {
	var text string
	if err := json.Unmarshal(payload, &text); err != nil {
		return err
	}
	parsed, err := ParseDate(text)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

package source

import "fmt"

type DateRange struct {
	from Date
	to   Date
}

func NewDateRange(from, to Date) (DateRange, error) {
	if from.IsZero() || to.IsZero() {
		return DateRange{}, errEmptyDate
	}
	if from.After(to) {
		return DateRange{}, fmt.Errorf("начало %s позже конца %s", from, to)
	}
	return DateRange{from: from, to: to}, nil
}

func ParseDateRange(from, to string) (DateRange, error) {
	parsedFrom, err := ParseDate(from)
	if err != nil {
		return DateRange{}, err
	}
	parsedTo, err := ParseDate(to)
	if err != nil {
		return DateRange{}, err
	}
	return NewDateRange(parsedFrom, parsedTo)
}

func (r DateRange) From() Date {
	return r.from
}

func (r DateRange) To() Date {
	return r.to
}

func (r DateRange) IsZero() bool {
	return r.from.IsZero() || r.to.IsZero()
}

func (r DateRange) Days() int {
	if r.IsZero() {
		return 0
	}
	return int(r.to.Time().Sub(r.from.Time()).Hours()/24) + 1
}

func (r DateRange) Contains(date Date) bool {
	return !date.Before(r.from) && !date.After(r.to)
}

func (r DateRange) Dates() []Date {
	dates := make([]Date, 0, r.Days())
	for current := r.from; !current.After(r.to); current = current.AddDays(1) {
		dates = append(dates, current)
	}
	return dates
}

func (r DateRange) String() string {
	return r.from.String() + ".." + r.to.String()
}

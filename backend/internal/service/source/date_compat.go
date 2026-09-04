package source

import (
	"errors"

	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

type Date = domainsource.Date
type DateRange = domainsource.DateRange

const DateLayout = domainsource.DateLayout

var NewDate = domainsource.NewDate
var ParseDate = domainsource.ParseDate
var DateFromTime = domainsource.DateFromTime
var NewDateRange = domainsource.NewDateRange
var ParseDateRange = domainsource.ParseDateRange

var errEmptyDate = errors.New("date is required")

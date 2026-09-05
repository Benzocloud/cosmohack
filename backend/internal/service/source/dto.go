package source

import (
	"time"

	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

type weatherDTO struct {
	SourceID           string   `json:"source_id"`
	TemperatureMeanC   *float64 `json:"temperature_mean_c"`
	PrecipitationSumMM *float64 `json:"precipitation_sum_mm"`
}

type referenceDTO struct {
	SourceID           string  `json:"source_id"`
	Mean               float64 `json:"mean"`
	Std                float64 `json:"std"`
	ReferenceYears     int     `json:"n_reference_years"`
	TargetYearExcluded bool    `json:"target_year_excluded"`
}

type observationDTO struct {
	Date          domainsource.Date    `json:"date"`
	PrimaryNDVI   *float64             `json:"primary_ndvi"`
	Quality       domainsource.Quality `json:"quality"`
	NDVISourceID  *string              `json:"ndvi_source_id"`
	Interval      *rangeDTO            `json:"interval"`
	ValidFraction *float64             `json:"valid_fraction"`
	MissingReason *string              `json:"missing_reason"`
	Weather       *weatherDTO          `json:"weather"`
	Reference     *referenceDTO        `json:"reference"`
	Indices       *indicesDTO          `json:"indices,omitempty"`
}

type indicesDTO struct {
	S2NDVI *float64 `json:"s2_ndvi"`
	S2EVI  *float64 `json:"s2_evi"`
	S2NDWI *float64 `json:"s2_ndwi"`
}

type sourceDTO struct {
	ID          string            `json:"id"`
	Kind        domainsource.Kind `json:"kind"`
	Provider    string            `json:"provider"`
	Dataset     string            `json:"dataset"`
	Mapping     string            `json:"mapping"`
	License     *string           `json:"license"`
	RetrievedAt string            `json:"retrieved_at"`
}

type rangeDTO struct {
	From domainsource.Date `json:"from"`
	To   domainsource.Date `json:"to"`
}

type pointDTO struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

func newObservationDTO(observation domainsource.Observation) observationDTO {
	dto := observationDTO{
		Date:          observation.Date(),
		PrimaryNDVI:   observation.PrimaryNDVI(),
		Quality:       observation.Quality(),
		ValidFraction: observation.ValidFraction(),
	}
	if observation.NDVISourceID() != "" {
		sourceID := observation.NDVISourceID()
		dto.NDVISourceID = &sourceID
	}
	if interval := observation.Interval(); interval != nil {
		dto.Interval = &rangeDTO{From: interval.From(), To: interval.To()}
	}
	if reason := observation.MissingReason(); reason != "" {
		dto.MissingReason = &reason
	}
	if weather := observation.Weather(); weather != nil {
		dto.Weather = &weatherDTO{
			SourceID:           weather.SourceID(),
			TemperatureMeanC:   weather.TemperatureMeanC(),
			PrecipitationSumMM: weather.PrecipitationSumMM(),
		}
	}
	if reference := observation.Reference(); reference != nil {
		dto.Reference = &referenceDTO{
			SourceID:           reference.SourceID(),
			Mean:               reference.Mean(),
			Std:                reference.Std(),
			ReferenceYears:     reference.ReferenceYears(),
			TargetYearExcluded: reference.TargetYearExcluded(),
		}
	}
	if indices := observation.Indices(); indices != nil {
		dto.Indices = &indicesDTO{
			S2NDVI: indices.S2NDVI,
			S2EVI:  indices.S2EVI,
			S2NDWI: indices.S2NDWI,
		}
	}
	return dto
}

func newSourceDTO(descriptor domainsource.Descriptor) sourceDTO {
	return sourceDTO{
		ID:          descriptor.ID(),
		Kind:        descriptor.Kind(),
		Provider:    descriptor.Provider(),
		Dataset:     descriptor.Dataset(),
		Mapping:     descriptor.Mapping(),
		License:     descriptor.License(),
		RetrievedAt: descriptor.RetrievedAt().Format(time.RFC3339),
	}
}

package source

import (
	"fmt"
	"time"
)

const MaxIdentifierLength = 128

type Kind string

const (
	KindSatellite Kind = "satellite"
	KindWeather   Kind = "weather"
	KindReference Kind = "reference"
)

func (k Kind) Valid() bool {
	switch k {
	case KindSatellite, KindWeather, KindReference:
		return true
	default:
		return false
	}
}

type Descriptor struct {
	id          string
	kind        Kind
	provider    string
	dataset     string
	mapping     string
	license     *string
	retrievedAt time.Time
}

type DescriptorSpec struct {
	ID          string
	Kind        Kind
	Provider    string
	Dataset     string
	Mapping     string
	License     *string
	RetrievedAt time.Time
}

func NewDescriptor(spec DescriptorSpec) (Descriptor, error) {
	if err := requireIdentifier("source id", spec.ID); err != nil {
		return Descriptor{}, err
	}
	if !spec.Kind.Valid() {
		return Descriptor{}, fmt.Errorf("source kind %q is unsupported", spec.Kind)
	}
	for label, value := range map[string]string{"provider": spec.Provider, "dataset": spec.Dataset, "mapping": spec.Mapping} {
		if value == "" {
			return Descriptor{}, fmt.Errorf("source %s field is required for %s", label, spec.ID)
		}
	}
	if spec.RetrievedAt.IsZero() {
		return Descriptor{}, fmt.Errorf("source retrieval time is required for %s", spec.ID)
	}
	license := spec.License
	if license != nil {
		copied := *license
		license = &copied
	}
	return Descriptor{
		id:          spec.ID,
		kind:        spec.Kind,
		provider:    spec.Provider,
		dataset:     spec.Dataset,
		mapping:     spec.Mapping,
		license:     license,
		retrievedAt: spec.RetrievedAt.UTC(),
	}, nil
}

func (d Descriptor) ID() string {
	return d.id
}

func (d Descriptor) Kind() Kind {
	return d.kind
}

func (d Descriptor) Provider() string {
	return d.provider
}

func (d Descriptor) Dataset() string {
	return d.dataset
}

func (d Descriptor) Mapping() string {
	return d.mapping
}

func (d Descriptor) License() *string {
	if d.license == nil {
		return nil
	}
	copied := *d.license
	return &copied
}

func (d Descriptor) RetrievedAt() time.Time {
	return d.retrievedAt
}

func (d Descriptor) IsZero() bool {
	return d.id == ""
}

func requireIdentifier(label, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(value) > MaxIdentifierLength {
		return fmt.Errorf("%s exceeds %d characters", label, MaxIdentifierLength)
	}
	return nil
}

// RequireIdentifier проверяет публичный идентификатор источника.
func RequireIdentifier(label, value string) error { return requireIdentifier(label, value) }

func requireSourceIdentifier(value string) error {
	return requireIdentifier("source identifier", value)
}

func License(value string) *string {
	return &value
}

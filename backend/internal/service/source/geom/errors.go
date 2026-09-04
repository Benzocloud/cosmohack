package geom

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeInvalidCoordinate ErrorCode = "invalid_coordinate"
	CodeRingNotClosed     ErrorCode = "ring_not_closed"
	CodeTooFewVertices    ErrorCode = "too_few_vertices"
	CodeTooManyVertices   ErrorCode = "too_many_vertices"
	CodeSelfIntersection  ErrorCode = "self_intersection"
	CodeDegenerateArea    ErrorCode = "degenerate_area"
	CodeAntimeridianSpan  ErrorCode = "antimeridian_span"
	CodeInvalidBBox       ErrorCode = "invalid_bbox"
	CodeUnsupportedShape  ErrorCode = "unsupported_shape"
	CodeMalformedGeoJSON  ErrorCode = "malformed_geojson"
)

type ValidationError struct {
	code   ErrorCode
	detail string
}

func NewValidationError(code ErrorCode, format string, args ...any) *ValidationError {
	return &ValidationError{code: code, detail: fmt.Sprintf(format, args...)}
}

func (e *ValidationError) Code() ErrorCode {
	return e.code
}

func (e *ValidationError) Error() string {
	return string(e.code) + ": " + e.detail
}

func (e *ValidationError) Is(target error) bool {
	other, ok := target.(*ValidationError)
	return ok && other.code == e.code
}

func CodeOf(err error) ErrorCode {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.code
	}
	return ""
}

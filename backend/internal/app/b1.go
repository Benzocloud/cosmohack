package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	geom "github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	analysisusecase "github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

// b1Collector adapts the source service to the analysis service port. The
// source package owns provider DTOs and snapshots; analysis receives only the
// canonical domain request.
type b1Collector struct {
	collector *source.Collector
	builder   *source.AnalyzeRequestBuilder
}

func newB1Collector(collector *source.Collector) *b1Collector {
	return &b1Collector{collector: collector, builder: source.NewAnalyzeRequestBuilder(0, 0)}
}

func (c *b1Collector) Collect(ctx context.Context, job domain.Job, area domain.Area, report analysisusecase.StageReporter) (analysisusecase.Collected, error) {
	polygon, err := domainPolygon(area.Geometry)
	if err != nil {
		return analysisusecase.Collected{}, fmt.Errorf("decode area geometry: %w", err)
	}

	period, err := domainsource.ParseDateRange(area.Period.From, area.Period.To)
	if err != nil {
		return analysisusecase.Collected{}, fmt.Errorf("parse area period: %w", err)
	}

	request, err := source.NewCollectRequest(area.ID, polygon, period)
	if err != nil {
		return analysisusecase.Collected{}, err
	}

	snapshot, err := c.collector.CollectWithStage(ctx, request, func(stage string) {
		if report != nil {
			report(stage)
		}
	})
	if err != nil {
		return analysisusecase.Collected{}, err
	}

	analysisRequest, err := c.builder.BuildDomain(snapshot, job.ID)
	if err != nil {
		return analysisusecase.Collected{}, fmt.Errorf("build analysis request: %w", err)
	}

	return analysisusecase.Collected{
		Request:    *analysisRequest,
		Provenance: map[string]any{"collector": "b1", "snapshot_revision": snapshot.Revision()},
	}, nil
}

// b1ContourFinder maps domain source results to the narrow HTTP handler port.
// HTTP response DTOs stay in handler and provider details stay in integration.
type b1ContourFinder struct {
	finder domainsource.ContourFinder
}

func (f b1ContourFinder) Find(ctx context.Context, minLon, minLat, maxLon, maxLat float64) ([]handler.Contour, error) {
	bbox, err := geom.NewBBox(minLon, minLat, maxLon, maxLat)
	if err != nil {
		return nil, err
	}

	result, err := f.finder.FindContours(ctx, bbox)
	if err != nil {
		return nil, err
	}

	contours := make([]handler.Contour, 0, result.Count())
	for _, contour := range result.Contours() {
		geometry, err := domainGeometry(contour.Polygon())
		if err != nil {
			return nil, fmt.Errorf("encode contour %s geometry: %w", contour.ID(), err)
		}

		contours = append(contours, handler.Contour{
			ID:       contour.ID(),
			Geometry: geometry,
			Source: handler.ContourSource{
				Provider:    contour.Origin().Provider(),
				Attribution: contour.Origin().Attribution(),
			},
		})
	}

	return contours, nil
}

func domainPolygon(polygon domain.Polygon) (*geom.Polygon, error) {
	if polygon.Type != "Polygon" || len(polygon.Coordinates) != 1 {
		return nil, fmt.Errorf("unsupported geometry type %q", polygon.Type)
	}

	ring := make([]geom.Coordinate, 0, len(polygon.Coordinates[0]))
	for index, pair := range polygon.Coordinates[0] {
		if len(pair) != 2 {
			return nil, fmt.Errorf("geometry point %d is not a coordinate pair", index)
		}

		coordinate, err := geom.NewCoordinate(pair[0], pair[1])
		if err != nil {
			return nil, err
		}

		ring = append(ring, coordinate)
	}

	return geom.NewPolygon(ring)
}

func domainGeometry(polygon *geom.Polygon) (domain.Polygon, error) {
	if polygon == nil {
		return domain.Polygon{}, errors.New("polygon is nil")
	}

	ring := polygon.Ring()

	coordinates := make([][]float64, 0, len(ring))
	for _, point := range ring {
		coordinates = append(coordinates, []float64{point.Lon(), point.Lat()})
	}

	return domain.Polygon{Type: "Polygon", Coordinates: [][][]float64{coordinates}}, nil
}

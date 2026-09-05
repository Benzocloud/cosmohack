package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	geom "github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	analysisusecase "github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

// b1Collector адаптирует сервис источников к порту сервиса анализа. Пакет
// source владеет DTO провайдеров и снимками; анализ получает только
// канонический доменный запрос.
type b1Collector struct {
	collector *source.Collector
	builder   *source.AnalyzeRequestBuilder
	areas     peerReader
}

type peerReader interface {
	ListAreas(context.Context) ([]domain.Area, error)
	GetResult(context.Context, string, string) (domain.AnalysisRecord, error)
}

func newB1Collector(collector *source.Collector, areas peerReader) *b1Collector {
	return &b1Collector{collector: collector, builder: source.NewAnalyzeRequestBuilder(0, 0), areas: areas}
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
	peerFailures := 0
	if analysisRequest.FeatureProfile == domain.FeatureProfileMultisensorV1 {
		analysisRequest.AreaContext = areaContext(area.Source.CropType)
		analysisRequest.Peers, peerFailures = c.peers(ctx, area)
	}

	return analysisusecase.Collected{
		Request: *analysisRequest,
		Provenance: map[string]any{
			"collector":            "b1",
			"snapshot_revision":    snapshot.Revision(),
			"crop_type_provided":   analysisRequest.AreaContext != nil,
			"peer_count":           len(analysisRequest.Peers),
			"peer_lookup_failures": peerFailures,
		},
	}, nil
}

func areaContext(cropType *string) *domain.AreaContext {
	if cropType == nil {
		return nil
	}
	crop := strings.TrimSpace(*cropType)
	if crop == "" {
		return nil
	}
	return &domain.AreaContext{CropType: &crop}
}

const maxPeerDistanceKm = 60

type peerCandidate struct {
	area     domain.Area
	distance float64
}

func (c *b1Collector) peers(ctx context.Context, area domain.Area) ([]domain.PeerSeries, int) {
	if c.areas == nil {
		return nil, 0
	}
	polygon, err := domainPolygon(area.Geometry)
	if err != nil {
		return nil, 0
	}
	origin := polygon.RepresentativePoint()
	areas, err := c.areas.ListAreas(ctx)
	if err != nil {
		return nil, 1
	}
	candidates := make([]peerCandidate, 0, len(areas))
	for _, candidate := range areas {
		if candidate.ID == area.ID || candidate.ShownResultVersion == "" {
			continue
		}
		candidatePolygon, err := domainPolygon(candidate.Geometry)
		if err != nil {
			continue
		}
		point := candidatePolygon.RepresentativePoint()
		distance := distanceKm(origin, point)
		if distance > maxPeerDistanceKm {
			continue
		}
		candidates = append(candidates, peerCandidate{area: candidate, distance: distance})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			return candidates[i].area.ID < candidates[j].area.ID
		}
		return candidates[i].distance < candidates[j].distance
	})
	if len(candidates) > domain.MaxPeers {
		candidates = candidates[:domain.MaxPeers]
	}

	peers := make([]domain.PeerSeries, 0, len(candidates))
	failures := 0
	for _, candidate := range candidates {
		result, err := c.areas.GetResult(ctx, candidate.area.ID, candidate.area.ShownResultVersion)
		if err != nil {
			failures++
			continue
		}
		observations := make([]domain.PeerObservation, 0, len(result.Series))
		for _, point := range result.Series {
			if point.State != domain.StateObserved || point.PrimaryNDVI == nil {
				continue
			}
			observations = append(observations, domain.PeerObservation{
				Date: point.Date, PrimaryNDVI: point.PrimaryNDVI, Quality: domain.QualityUsable,
			})
		}
		if len(observations) > 0 {
			peers = append(peers, domain.PeerSeries{AreaID: candidate.area.ID, Observations: observations})
		}
	}
	return peers, failures
}

func distanceKm(a, b geom.Coordinate) float64 {
	const earthRadiusKm = 6371
	lat1, lat2 := a.Lat()*math.Pi/180, b.Lat()*math.Pi/180
	dLat := lat2 - lat1
	dLon := (b.Lon() - a.Lon()) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(h))
}

// b1ContourFinder переводит доменные результаты источников в узкий порт HTTP-обработчика.
// DTO HTTP-ответов остаются в handler, а детали провайдеров — в integration.
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

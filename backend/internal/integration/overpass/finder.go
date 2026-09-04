package overpass

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/httpx"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

const (
	ProviderName     = "openstreetmap-overpass"
	PrimaryEndpoint  = "https://overpass-api.de/api/interpreter"
	FallbackEndpoint = "https://overpass.kumi.systems/api/interpreter"
	Dataset          = "OpenStreetMap landuse=farmland (way)"
	License          = "ODbL-1.0"
	Attribution      = "© OpenStreetMap contributors"

	defaultQueryTimeout = 60
	defaultMaxResults   = 200
)

type Config struct {
	PrimaryEndpoint     string
	FallbackEndpoint    string
	QueryTimeoutSeconds int
	MaxResults          int
	Limits              source.Limits
	Client              *httpx.Client
	Clock               domain.Clock
}

func DefaultConfig() Config {
	return Config{
		PrimaryEndpoint:     PrimaryEndpoint,
		FallbackEndpoint:    FallbackEndpoint,
		QueryTimeoutSeconds: defaultQueryTimeout,
		MaxResults:          defaultMaxResults,
		Limits:              source.DefaultLimits(),
		Client:              httpx.NewClient(httpx.WithTimeout(90 * time.Second)),
		Clock:               time.Now,
	}
}

type Finder struct {
	failover *httpx.Failover
	query    *queryBuilder
	limits   source.Limits
	clock    domain.Clock
}

func NewFinder(config Config) (*Finder, error) {
	if config.PrimaryEndpoint == "" {
		config.PrimaryEndpoint = PrimaryEndpoint
	}
	if config.QueryTimeoutSeconds <= 0 {
		config.QueryTimeoutSeconds = defaultQueryTimeout
	}
	if config.MaxResults <= 0 {
		config.MaxResults = defaultMaxResults
	}
	if config.Client == nil {
		config.Client = httpx.NewClient(httpx.WithTimeout(90 * time.Second))
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.Limits == (source.Limits{}) {
		config.Limits = source.DefaultLimits()
	}
	endpoints, err := httpx.NewEndpoints(config.PrimaryEndpoint, config.FallbackEndpoint)
	if err != nil {
		return nil, err
	}
	failover, err := httpx.NewFailover(config.Client, endpoints)
	if err != nil {
		return nil, err
	}
	return &Finder{
		failover: failover,
		query:    newQueryBuilder(config.QueryTimeoutSeconds, config.MaxResults),
		limits:   config.Limits,
		clock:    config.Clock,
	}, nil
}

func (f *Finder) FindContours(ctx context.Context, bbox geom.BBox) (source.ContourSearchResult, error) {
	if err := f.limits.ValidateSearchArea(bbox); err != nil {
		return source.ContourSearchResult{}, err
	}
	query := f.query.build(bbox)
	document := &responseDocument{}
	if _, err := f.failover.DoJSON(ctx, ProviderName, requestFactory(query), document); err != nil {
		return source.ContourSearchResult{}, err
	}
	origin, err := source.NewOrigin(source.OriginSpec{
		Provider:        ProviderName,
		Dataset:         Dataset,
		License:         License,
		Attribution:     Attribution,
		Query:           query,
		UpstreamVersion: document.Osm3s.TimestampOsmBase,
		RetrievedAt:     f.clock().UTC(),
	})
	if err != nil {
		return source.ContourSearchResult{}, source.WrapProviderError(source.FailureMalformed, ProviderName, err,
			"происхождение контуров не построено")
	}
	contours, notes := f.convert(document, origin)
	truncated := len(document.Elements) >= f.query.maxResults
	if truncated {
		notes = append(notes, fmt.Sprintf("Показаны первые %d контуров области; уменьшите область поиска", f.query.maxResults))
	}
	if len(contours) == 0 {
		notes = append(notes, "Контуры landuse=farmland в выбранной области не найдены")
	}
	notes = append(notes, "landuse=farmland — кандидат по покрытию OpenStreetMap, а не юридическая граница поля")
	return source.NewContourSearchResult(bbox, origin, contours, truncated, notes), nil
}

func (f *Finder) convert(document *responseDocument, origin source.Origin) ([]source.Contour, []string) {
	contours := make([]source.Contour, 0, len(document.Elements))
	invalid, filtered := 0, 0
	for _, element := range document.Elements {
		if !element.isSupported() {
			continue
		}
		polygon, err := element.polygon()
		if err != nil {
			invalid++
			continue
		}
		if err := f.limits.ValidatePolygon(polygon); err != nil {
			filtered++
			continue
		}
		contour, err := source.NewContour(element.contourID(), element.name(), polygon, origin, element.Tags)
		if err != nil {
			invalid++
			continue
		}
		contours = append(contours, contour)
	}
	notes := make([]string, 0, 2)
	if invalid > 0 {
		notes = append(notes, fmt.Sprintf("Пропущено объектов с некорректной геометрией: %d", invalid))
	}
	if filtered > 0 {
		notes = append(notes, fmt.Sprintf("Пропущено объектов вне пределов площади участка: %d", filtered))
	}
	return contours, notes
}

func requestFactory(query string) httpx.RequestFactory {
	return func(ctx context.Context, endpoint string) (*http.Request, error) {
		form := url.Values{}
		form.Set("data", query)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return request, nil
	}
}

type queryBuilder struct {
	timeoutSeconds int
	maxResults     int
}

func newQueryBuilder(timeoutSeconds, maxResults int) *queryBuilder {
	return &queryBuilder{timeoutSeconds: timeoutSeconds, maxResults: maxResults}
}

func (b *queryBuilder) build(bbox geom.BBox) string {
	return fmt.Sprintf(
		"[out:json][timeout:%d];way[\"landuse\"=\"farmland\"](%s,%s,%s,%s);out geom %d;",
		b.timeoutSeconds,
		formatDegrees(bbox.MinLat()), formatDegrees(bbox.MinLon()),
		formatDegrees(bbox.MaxLat()), formatDegrees(bbox.MaxLon()),
		b.maxResults,
	)
}

func formatDegrees(value float64) string {
	return fmt.Sprintf("%.7f", value)
}

//go:build live

package cdse_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/cdse"
)

func TestLiveFetchesS2Indices(t *testing.T) {
	clientID, clientSecret := os.Getenv("CDSE_CLIENT_ID"), os.Getenv("CDSE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		t.Skip("CDSE access is not configured")
	}
	credentials, err := cdse.NewCredentials(clientID, clientSecret)
	if err != nil {
		t.Fatalf("credentials were not built: %v", err)
	}
	provider, err := cdse.NewProvider(cdse.DefaultConfig(credentials))
	if err != nil {
		t.Fatalf("provider was not built: %v", err)
	}
	polygon, err := geom.NewPolygon([]geom.Coordinate{
		geom.MustCoordinate(39.0, 45.25),
		geom.MustCoordinate(39.01, 45.25),
		geom.MustCoordinate(39.01, 45.26),
		geom.MustCoordinate(39.0, 45.26),
		geom.MustCoordinate(39.0, 45.25),
	})
	if err != nil {
		t.Fatalf("polygon was not built: %v", err)
	}
	period, err := source.ParseDateRange("2025-06-01", "2025-06-10")
	if err != nil {
		t.Fatalf("period was not built: %v", err)
	}
	request, err := source.NewSatelliteRequest(polygon, period)
	if err != nil {
		t.Fatalf("satellite request was not built: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	series, err := provider.FetchIndices(ctx, request)
	if err != nil {
		t.Fatalf("S2 indices were not fetched: %v", err)
	}
	if len(series.Samples()) == 0 {
		t.Fatal("S2 index series is empty")
	}
	t.Logf("S2 index intervals: %d; descriptor: %s", len(series.Samples()), series.Descriptor().Mapping())
}

package geom_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("..", "testdata", "geojson", name))
	if err != nil {
		t.Fatalf("фикстура %s не прочитана: %v", name, err)
	}
	return payload
}

func TestPolygonCodecDecodesSupportedDocuments(t *testing.T) {
	codec := geom.NewPolygonCodec(0)
	for _, name := range []string{"user_polygon_feature.json", "user_polygon_collection.json"} {
		t.Run(name, func(t *testing.T) {
			polygon, err := codec.Decode(fixture(t, name))
			if err != nil {
				t.Fatalf("документ не принят: %v", err)
			}
			if polygon.VertexCount() != 4 {
				t.Fatalf("вершин %d, ожидалось 4", polygon.VertexCount())
			}
			if !polygon.BBox().Contains(polygon.RepresentativePoint()) {
				t.Fatal("репрезентативная точка вне bbox")
			}
		})
	}
}

func TestPolygonCodecRejectsUnsupportedDocuments(t *testing.T) {
	codec := geom.NewPolygonCodec(0)
	cases := map[string]struct {
		name string
		code geom.ErrorCode
	}{
		"мультиполигон": {name: "multipolygon.json", code: geom.CodeUnsupportedShape},
		"с отверстием":  {name: "polygon_with_hole.json", code: geom.CodeUnsupportedShape},
		"порядок осей":  {name: "swapped_axis_order.json", code: geom.CodeInvalidCoordinate},
	}
	for title, testCase := range cases {
		t.Run(title, func(t *testing.T) {
			if _, err := codec.Decode(fixture(t, testCase.name)); geom.CodeOf(err) != testCase.code {
				t.Fatalf("код ошибки %q, ожидался %q (ошибка %v)", geom.CodeOf(err), testCase.code, err)
			}
		})
	}
}

func TestPolygonCodecRejectsMalformedInput(t *testing.T) {
	codec := geom.NewPolygonCodec(0)
	cases := map[string][]byte{
		"пусто":         []byte("   "),
		"не json":       []byte("{"),
		"без типа":      []byte(`{"coordinates": []}`),
		"пустое кольцо": []byte(`{"type": "Polygon", "coordinates": []}`),
	}
	for title, payload := range cases {
		t.Run(title, func(t *testing.T) {
			if _, err := codec.Decode(payload); geom.CodeOf(err) != geom.CodeMalformedGeoJSON {
				t.Fatalf("код ошибки %q (ошибка %v)", geom.CodeOf(err), err)
			}
		})
	}
}

func TestPolygonCodecLimitsDocumentSize(t *testing.T) {
	codec := geom.NewPolygonCodec(16)
	if _, err := codec.Decode(fixture(t, "user_polygon_feature.json")); geom.CodeOf(err) != geom.CodeMalformedGeoJSON {
		t.Fatalf("превышение предела размера не отклонено: %v", err)
	}
}

func TestPolygonCodecRoundTrip(t *testing.T) {
	codec := geom.NewPolygonCodec(0)
	original, err := codec.Decode(fixture(t, "user_polygon_feature.json"))
	if err != nil {
		t.Fatalf("документ не принят: %v", err)
	}
	encoded, err := codec.Encode(original)
	if err != nil {
		t.Fatalf("полигон не сериализован: %v", err)
	}
	restored, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("сериализованный полигон не разобран: %v", err)
	}
	originalRing, restoredRing := original.Ring(), restored.Ring()
	if len(originalRing) != len(restoredRing) {
		t.Fatalf("длина кольца %d против %d", len(restoredRing), len(originalRing))
	}
	for index := range originalRing {
		if !originalRing[index].Equal(restoredRing[index]) {
			t.Fatalf("точка %d изменилась: %v против %v", index, restoredRing[index], originalRing[index])
		}
	}
}

import { bbox as turfBbox } from '@turf/bbox';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { ExpressionSpecification, FilterSpecification } from 'maplibre-gl';
import { useEffect, useMemo, useRef, useState } from 'react';
import MapGL, { Layer, Source, type MapRef } from 'react-map-gl/maplibre';

import type { Contour, ContoursResult } from '@/api/adapters/contours';
import type { AppError } from '@/api/client';
import { useAreas, useContours, useLimits } from '@/api/queries';
import { cssVar } from '@/lib/chart-theme';
import { polygonAreaHa, validatePolygon } from '@/lib/geo';
import { useDraft } from '@/store/draft';
import { selectionActions, useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';
import { AddAreaDialog, type AddAreaSource } from './AddAreaDialog';
import { BasemapSwitcher } from './BasemapSwitcher';
import { ContourPopover, type ContourPopoverState } from './ContourPopover';
import { ContoursButton } from './ContoursButton';
import { DrawToolbar } from './DrawToolbar';
import { MapEmptyHint } from './MapEmptyHint';
import { MapLegend } from './MapLegend';
import {
  DEFAULT_MAP_VIEW,
  MOCK_MAP_VIEW,
  OPENFREEMAP_STYLE_URL,
  POSITRON_STYLE_URL,
  SATELLITE_STYLE,
  readLastMapView,
  saveLastMapView,
} from './map-style';
import { type Ring, useTerraDraw } from './useTerraDraw';

/**
 * Карта (frontend-plan §8): подложка + векторные полигоны, terra-draw для рисования,
 * никакой растровой статистики NDVI без подтверждённого источника.
 *
 * Порядок слоёв: contours-fill → contours-line → areas-fill → areas-line →
 * areas-line-none → selected-halo → selected-line → слои terra-draw.
 * Цвет полигона = итог сохранённого lastResult за подписанный период; перекраска
 * по выбранной дате графика не делается (бриф §3C).
 *
 * Задел под P2: источник 'ndvi-raster' добавляется ТОЛЬКО если бэкенд отдаст URL
 * слоя (сейчас всегда null — ничего фейкового не рисуем, вопрос в ui-spec §8.2).
 */
const NDVI_RASTER_URL: string | null = null;

// Цвета статусов — из токенов §2.2; альфа заливки 0.22 ≈ 0x38 в 8-значном hex
const VERDICT_FILL_ALPHA = '38';
const hex = (name: string, fallback: string) => cssVar(name, fallback).replace('#', '');

const verdictColorExpr = (alpha: string): ExpressionSpecification => [
  'match',
  ['get', 'verdict'],
  'normal',
  `#${hex('--verdict-normal', '1B7F3B')}${alpha}`,
  'candidate',
  `#${hex('--verdict-candidate', 'C77700')}${alpha}`,
  'confirmed',
  `#${hex('--verdict-confirmed', 'C8102E')}${alpha}`,
  'insufficient_data',
  `#${hex('--verdict-insufficient', '6B7785')}${alpha}`,
  `#${hex('--verdict-none', 'A8B3C0')}00`,
];

export function MapView({ variant = 'desktop' }: { variant?: 'desktop' | 'mobile' }) {
  const demoMode = useUi((s) => s.demoMode);
  const basemap = useUi((s) => s.basemap);
  const drawMode = useDraft((s) => s.drawMode);
  const vertices = useDraft((s) => s.vertices);
  const setVertices = useDraft((s) => s.setVertices);
  const cancelDrawing = useDraft((s) => s.cancelDrawing);
  const setFinishedGeometry = useDraft((s) => s.setFinishedGeometry);
  const selectedAreaId = useSelection((s) => s.selectedAreaId);
  const selectionSource = useSelection((s) => s.selectionSource);

  const [mapInstance, setMapInstance] = useState<maplibregl.Map | null>(null);
  const mapRef = useRef<MapRef>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const areasQuery = useAreas();
  const areas = areasQuery.data ?? [];

  const [bbox, setBbox] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const [styleBroken, setStyleBroken] = useState(false);
  // dev-удобство: ?mock_case=empty|failed переключает сценарий фикстур контуров
  const mockCase = new URLSearchParams(window.location.search).get('mock_case') ?? undefined;
  const contoursQuery = useContours(bbox, mockCase);
  const contoursResult: ContoursResult | null = contoursQuery.data ?? null;

  const [contourPopover, setContourPopover] = useState<ContourPopoverState | null>(null);
  const [addSource, setAddSource] = useState<AddAreaSource | null>(null);
  const [addOpen, setAddOpen] = useState(false);

  const drawing = drawMode === 'drawing';
  // В моке стартуем строго над фикстурными контурами; иначе — сохранённый вид/центр РФ
  const initialView = useMemo(
    () => (demoMode ? MOCK_MAP_VIEW : (readLastMapView() ?? DEFAULT_MAP_VIEW)),
    [demoMode],
  );

  // --- черновик и terra-draw ---
  const handleFinish = (ring: Ring) => {
    setFinishedGeometry({ type: 'Polygon', coordinates: [[...ring, ring[0]]] });
  };
  const draw = useTerraDraw({
    map: mapInstance,
    drawing,
    onVerticesChange: setVertices,
    onFinish: handleFinish,
  });

  // Esc отменяет рисование (бриф §3A: завершение и отмена доступны)
  useEffect(() => {
    if (!drawing) return undefined;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        draw.clear();
        cancelDrawing();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [drawing, draw, cancelDrawing]);

  // Выбор из списка → fitBounds; выбор кликом по карте — без подлёта (бриф §3C)
  const prevSelected = useRef<string | null>(null);
  useEffect(() => {
    if (
      selectedAreaId &&
      selectedAreaId !== prevSelected.current &&
      selectionSource === 'list' &&
      mapInstance
    ) {
      const area = areas.find((item) => item.id === selectedAreaId);
      if (area) {
        const [west, south, east, north] = turfBbox(area.geometry);
        mapInstance.fitBounds(
          [
            [west, south],
            [east, north],
          ],
          { padding: 48, maxZoom: 15, duration: 400 },
        );
      }
    }
    prevSelected.current = selectedAreaId;
  }, [selectedAreaId, selectionSource, areas, mapInstance]);

  const searchContours = () => {
    const map = mapRef.current;
    if (!map) return;
    const bounds = map.getBounds();
    const { _sw: sw, _ne: ne } = bounds as unknown as {
      _sw: { lng: number; lat: number };
      _ne: { lng: number; lat: number };
    };
    setBbox(`${sw.lng},${sw.lat},${ne.lng},${ne.lat}`);
    setStale(false);
    setContourPopover(null);
  };

  const startDrawing = () => {
    setContourPopover(null);
    // стартуем «чисто»: предыдущий черновик не восстанавливаем
    draw.clear();
    useDraft.getState().startDrawing();
  };

  const finishDrawing = () => {
    // по 'finish' прилетит finishedGeometry → откроем диалог (эффект ниже)
    draw.finishDrawing();
  };

  // Завершённый черновик → диалог добавления
  const finishedGeometry = useDraft((s) => s.finishedGeometry);
  useEffect(() => {
    if (!finishedGeometry) return;
    setAddSource({ kind: 'drawn', label: 'Нарисован вручную', geometry: finishedGeometry });
    setAddOpen(true);
  }, [finishedGeometry]);

  const openAddFromContour = (contour: Contour) => {
    setAddSource({
      kind: 'contour',
      label: 'Контур OpenStreetMap',
      externalId: contour.externalId ?? contour.id,
      name: contour.name,
      geometry: contour.geometry,
    });
    setAddOpen(true);
  };

  const closeAddDialog = () => {
    setAddOpen(false);
    // чертёжные фичи больше не нужны: участок рендерится отдельным слоем
    draw.clear();
    useDraft.getState().cancelDrawing();
  };

  const handleSaved = (
    area: { id: string },
    startError: AppError | null,
    period: { from: string; to: string },
  ) => {
    closeAddDialog();
    if (startError) {
      // «Сохранено, запуск не удался» → участок остаётся с кнопкой «Запустить анализ»
      useUi.getState().setPendingStart({ areaId: area.id, period });
    }
    selectionActions.selectArea(area.id, 'list');
  };

  const validationVertices: Ring = vertices;
  const limits = useLimits().data ?? null;
  const validation = useMemo(
    () => validatePolygon(validationVertices, limits),
    [validationVertices, limits],
  );

  const areasFC = useMemo(
    () => ({
      type: 'FeatureCollection' as const,
      features: areas.map((area) => ({
        type: 'Feature' as const,
        geometry: area.geometry,
        properties: {
          areaId: area.id,
          verdict: area.lastResult?.verdict ?? 'none',
        },
      })),
    }),
    [areas],
  );

  const contoursFC = useMemo(
    () => ({
      type: 'FeatureCollection' as const,
      features: (contoursResult?.contours ?? []).map((contour) => ({
        type: 'Feature' as const,
        geometry: contour.geometry,
        properties: { contourId: contour.id },
      })),
    }),
    [contoursResult],
  );

  const mapStyle = styleBroken
    ? OPENFREEMAP_STYLE_URL
    : basemap === 'satellite'
      ? SATELLITE_STYLE
      : POSITRON_STYLE_URL;
  const alwaysAttribution =
    'Контуры: © OpenStreetMap contributors | © OpenStreetMap contributors © CARTO';

  return (
    <div className="relative h-full min-h-0 w-full" ref={containerRef}>
      <MapGL
        ref={mapRef}
        mapLib={maplibregl}
        mapStyle={mapStyle}
        initialViewState={initialView}
        style={{ width: '100%', height: '100%' }}
        attributionControl={false}
        onLoad={(event) => {
          // своя атрибуция: OSM/CARTO и контуры OSM видны на обеих подложках (уточнение FE-3)
          event.target.addControl(
            new maplibregl.AttributionControl({
              customAttribution: alwaysAttribution,
              compact: variant === 'mobile',
            }),
          );
          if (import.meta.env.DEV) {
            (window as unknown as { __agroMap?: maplibregl.Map }).__agroMap = event.target;
          }
          setMapInstance(event.target);
        }}
        onError={() => {
          // сбой подложки Positron → разово уходим на запасной стиль (§1)
          if (!styleBroken && basemap === 'map') setStyleBroken(true);
        }}
        onMoveEnd={(event) => {
          const { longitude, latitude, zoom } = event.viewState;
          saveLastMapView({ longitude, latitude, zoom });
          if (bbox) setStale(true);
        }}
        onClick={(event) => {
          if (drawing) return; // клики в режиме рисования обрабатывает terra-draw
          const areasFeatures = event.target.queryRenderedFeatures(event.point, {
            layers: ['areas-fill'],
          });
          const areaId = areasFeatures[0]?.properties?.areaId;
          if (typeof areaId === 'string') {
            selectionActions.selectArea(areaId, 'map');
            return;
          }
          const contourFeatures = event.target.queryRenderedFeatures(event.point, {
            layers: ['contours-fill'],
          });
          const contourId = contourFeatures[0]?.properties?.contourId;
          if (typeof contourId === 'string' && contoursResult) {
            const contour = contoursResult.contours.find((item) => item.id === contourId);
            if (contour) {
              // точка клика ограничивается контейнером карты, чтобы карточка не вылезала
              const rect = containerRef.current?.getBoundingClientRect();
              const px = Math.max(8, Math.min(event.point.x, (rect?.width ?? 320) - 270));
              const py = Math.max(8, Math.min(event.point.y, (rect?.height ?? 240) - 240));
              setContourPopover({
                contour,
                areaHa: polygonAreaHa(contourToRing(contour.geometry)).toFixed(1),
                point: { x: px, y: py },
              });
            }
          }
        }}
      >
        {/* P2: растровый слой NDVI — только по URL от B1/B4 (ui-spec §8.2) */}
        {NDVI_RASTER_URL && (
          <Source id="ndvi-raster" type="raster" tiles={[NDVI_RASTER_URL]} tileSize={256}>
            <Layer
              id="ndvi-raster-layer"
              type="raster"
              beforeId="areas-fill"
              paint={{ 'raster-opacity': 0.7 }}
            />
          </Source>
        )}

        <Source id="contours" type="geojson" data={contoursFC}>
          <Layer
            id="contours-fill"
            type="fill"
            paint={{ 'fill-color': cssVar('--contour-found-fill', 'rgba(45,91,227,.08)') }}
          />
          <Layer
            id="contours-line"
            type="line"
            paint={{
              'line-color': cssVar('--contour-found', '#2D5BE3'),
              'line-width': 1.5,
              'line-dasharray': [2, 2],
            }}
          />
        </Source>

        <Source id="areas" type="geojson" data={areasFC}>
          <Layer
            id="areas-fill"
            type="fill"
            paint={{ 'fill-color': verdictColorExpr(VERDICT_FILL_ALPHA) }}
          />
          <Layer
            id="areas-line"
            type="line"
            filter={['!=', ['get', 'verdict'], 'none'] as FilterSpecification}
            paint={{ 'line-color': verdictColorExpr('FF'), 'line-width': 2 }}
          />
          <Layer
            id="areas-line-none"
            type="line"
            filter={['==', ['get', 'verdict'], 'none'] as FilterSpecification}
            paint={{
              'line-color': cssVar('--verdict-none', '#A8B3C0'),
              'line-width': 2,
              'line-dasharray': [2, 2],
            }}
          />
          {/* выбранный: белое гало + тёмная обводка независимо от статуса (§8) */}
          <Layer
            id="areas-selected-halo"
            type="line"
            filter={['==', ['get', 'areaId'], selectedAreaId ?? '—'] as FilterSpecification}
            paint={{ 'line-color': '#FFFFFF', 'line-width': 6 }}
          />
          <Layer
            id="areas-selected-line"
            type="line"
            filter={['==', ['get', 'areaId'], selectedAreaId ?? '—'] as FilterSpecification}
            paint={{ 'line-color': cssVar('--area-selected-outline', '#151A21'), 'line-width': 3 }}
          />
        </Source>
      </MapGL>

      {/* Управление поиском/рисованием — 44px цели */}
      {/* Колонка управления: не заезжает на переключатель подложки справа */}
      <div className="absolute left-3 top-3 z-10 flex w-[calc(100%-124px)] flex-col gap-2">
        <ContoursButton
          loading={contoursQuery.isFetching}
          stale={stale}
          result={contoursResult}
          onSearch={searchContours}
          onDraw={startDrawing}
        />
      </div>

      <div className="absolute right-3 top-3 z-10">
        <BasemapSwitcher />
      </div>

      {areas.length === 0 && !drawing && (
        <MapEmptyHint onSearch={searchContours} onDraw={startDrawing} />
      )}

      {drawing && (
        <DrawToolbar
          vertexCount={vertices.length}
          validation={validation}
          onUndo={() => draw.undoVertex()}
          onFinish={finishDrawing}
          onCancel={() => {
            draw.clear();
            cancelDrawing();
          }}
        />
      )}

      <MapLegend variant={variant} />

      <ContourPopover
        state={contourPopover}
        onClose={() => setContourPopover(null)}
        onAdd={openAddFromContour}
        variant={variant}
      />

      <AddAreaDialog
        source={addSource}
        open={addOpen}
        areasCount={areas.length}
        onOpenChange={(open) => {
          if (!open) closeAddDialog();
        }}
        onSaved={handleSaved}
      />
    </div>
  );
}

function contourToRing(geometry: GeoJSON.Polygon | GeoJSON.MultiPolygon): Ring {
  return (
    geometry.type === 'Polygon' ? geometry.coordinates[0] : geometry.coordinates[0][0]
  ) as Ring;
}

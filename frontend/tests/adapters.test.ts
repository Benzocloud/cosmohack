import { adaptArea, adaptAreaList, adaptResultMeta } from '@/api/adapters/areas';
import { adaptContours } from '@/api/adapters/contours';
import { adaptEvent, adaptEventList } from '@/api/adapters/events';
import { adaptJob } from '@/api/adapters/jobs';
import { adaptLimits, adaptLimitsFromErrorBody } from '@/api/adapters/limits';
import { adaptSeries, adaptSeriesPoint } from '@/api/adapters/series';
import { normalizeError, normalizeNetworkError } from '@/api/client';
import { describe, expect, it } from 'vitest';

/**
 * Unit-тесты маппинга адаптеров (frontend-plan §10 FE-1):
 * empty/failed различаются; null NDVI не превращается в 0; отсутствующие
 * необязательные поля → undefined/null, не выдуманные значения.
 */

const polygon: GeoJSON.Polygon = {
  type: 'Polygon',
  coordinates: [
    [
      [38.9, 45.55],
      [38.98, 45.55],
      [38.98, 45.61],
      [38.9, 45.61],
      [38.9, 45.55],
    ],
  ],
};

describe('adaptContours', () => {
  it('различает empty и failed', () => {
    const empty = adaptContours({ status: 'empty', source: 'OSM', features: [] });
    const failed = adaptContours({ status: 'failed', source: 'OSM', features: [] });
    expect(empty.status).toBe('empty');
    expect(failed.status).toBe('failed');
  });

  it('маппит features и coverage_note', () => {
    const result = adaptContours({
      status: 'ok',
      source: 'OSM Overpass',
      coverage_note: 'Каталог неполный',
      features: [
        { geometry: polygon, properties: { id: 'way/1', name: 'Контур 1', external_id: 'way/1' } },
        { geometry: polygon, properties: { name: 'без id' } }, // некорректная запись — пропускается
        { properties: { id: 'way/2' } }, // без геометрии — пропускается
      ],
    });
    expect(result.status).toBe('ok');
    expect(result.contours).toHaveLength(1);
    expect(result.contours[0].id).toBe('way/1');
    expect(result.coverageNote).toBe('Каталог неполный');
  });

  it('неизвестный статус трактуется как failed', () => {
    expect(adaptContours({}).status).toBe('failed');
  });
});

describe('adaptArea / adaptResultMeta', () => {
  const validRaw = {
    id: 'a1',
    name: 'Поле',
    geometry: polygon,
    source: { kind: 'contour', label: 'Контур OpenStreetMap' },
    created_at: '2026-09-04T08:00:00Z',
    last_result: null,
    active_job: null,
  };

  it('отсутствующие необязательные поля → undefined', () => {
    const area = adaptArea(validRaw);
    expect(area.lastResult).toBeUndefined();
    expect(area.activeJob).toBeUndefined();
    expect(area.source.externalId).toBeUndefined();
  });

  it('сохраняет null severity и drawn-источник', () => {
    const meta = adaptResultMeta({
      result_version: 'v1',
      period: { from: '2024-03-01', to: '2024-09-30' },
      computed_at: '2026-09-04T08:05:00Z',
      verdict: 'confirmed',
      severity: null,
      sources: { sentinel2: { status: 'ok' } },
      limitations: [],
    });
    expect(meta.severity).toBeNull();
    expect(meta.verdict).toBe('confirmed');
  });

  it('неизвестный verdict — ошибка адаптера, не подмена', () => {
    expect(() =>
      adaptResultMeta({
        result_version: 'v1',
        period: { from: '2024-03-01', to: '2024-09-30' },
        computed_at: 'x',
        verdict: 'healthy',
        sources: {},
        limitations: [],
      }),
    ).toThrow(/verdict/);
  });

  it('adaptAreaList принимает и массив, и {areas: [...]}', () => {
    expect(adaptAreaList([validRaw])).toHaveLength(1);
    expect(adaptAreaList({ areas: [validRaw] })).toHaveLength(1);
  });
});

describe('adaptJob', () => {
  it('маппит поля и сохраняет null стадии', () => {
    const job = adaptJob({
      id: 'j1',
      area_id: 'a1',
      requested_period: { from: '2024-03-01', to: '2024-09-30' },
      status: 'failed',
      stage: null,
      error_code: 'interrupted',
      error_message: 'interrupted',
      result_version: null,
      updated_at: '2026-09-04T08:16:00Z',
    });
    expect(job.status).toBe('failed');
    expect(job.stage).toBeNull();
    expect(job.errorMessage).toBe('interrupted');
    expect(job.resultVersion).toBeNull();
  });

  it('неизвестный статус — ошибка адаптера', () => {
    expect(() =>
      adaptJob({
        id: 'j1',
        area_id: 'a1',
        requested_period: { from: 'x', to: 'y' },
        status: 'pending_forever',
        updated_at: 'x',
      }),
    ).toThrow(/статус/);
  });
});

describe('adaptSeries / adaptSeriesPoint', () => {
  it('null NDVI → provenance missing, значение не становится 0', () => {
    const point = adaptSeriesPoint({ date: '2024-05-01', ndvi: null });
    expect(point.ndvi).toBeNull();
    expect(point.provenance).toBe('missing');
    expect(point.ndvi).not.toBe(0);
  });

  it('число без provenance → observed', () => {
    expect(adaptSeriesPoint({ date: 'd', ndvi: 0.42 }).provenance).toBe('observed');
  });

  it('переданный валидный provenance сохраняется, NaN → missing', () => {
    expect(adaptSeriesPoint({ date: 'd', ndvi: 0.4, provenance: 'imputed' }).provenance).toBe(
      'imputed',
    );
    expect(adaptSeriesPoint({ date: 'd', ndvi: Number.NaN }).provenance).toBe('missing');
  });

  it('настоящий ноль NDVI остаётся нулём', () => {
    const point = adaptSeriesPoint({ date: 'd', ndvi: 0 });
    expect(point.ndvi).toBe(0);
  });

  it('отклонение с неизвестной единицей отбрасывается (не выдумываем смысл)', () => {
    const point = adaptSeriesPoint({
      date: 'd',
      ndvi: 0.3,
      deviation: { value: -0.2, unit: 'bananas', base: 'фон' },
    });
    expect(point.deviation).toBeNull();
  });

  it('фон без mean → null; weather отсутствует → null', () => {
    const series = adaptSeries({
      area_id: 'a1',
      result_version: 'v1',
      period: { from: '2024-03-01', to: '2024-09-30' },
      points: [{ date: 'd', ndvi: 0.3, background: { low: 0.1 } }],
      weather: null,
    });
    expect(series.points[0].background).toBeNull();
    expect(series.weather).toBeNull();
    expect(series.background).toBeNull();
  });
});

describe('adaptEvent / adaptEventList', () => {
  const validRaw = {
    id: 'e1',
    period: { from: '2024-06-10', to: '2024-07-05' },
    verdict: 'confirmed',
    severity: 'high',
    detected: { magnitude: -0.21, unit: 'ndvi', base: 'фон', text: '−0.21' },
    basis: { observed_count: 14, imputed_count: 5, background_comparable: true },
    weather: { facts: [{ label: 'Осадки', value: '12 мм' }], hypothesis: null },
    limitations: [],
  };

  it('hypothesis null сохраняется (UI покажет «Причина не установлена»)', () => {
    const event = adaptEvent(validRaw);
    expect(event.weather?.hypothesis).toBeNull();
    expect(event.severity).toBe('high');
  });

  it('weather отсутствует → undefined', () => {
    const event = adaptEvent({ ...validRaw, weather: undefined });
    expect(event.weather).toBeUndefined();
  });

  it('неизвестный verdict события — ошибка адаптера', () => {
    expect(() => adaptEvent({ ...validRaw, verdict: 'critical' })).toThrow(/verdict/);
  });

  it('adaptEventList принимает {events: [...]}', () => {
    expect(adaptEventList({ events: [validRaw] })).toHaveLength(1);
  });
});

describe('adaptLimits', () => {
  it('полный набор чисел → Limits', () => {
    expect(
      adaptLimits({
        limits: {
          area_ha_max: 5000,
          vertices_max: 200,
          period_days_max: 366,
          min_date: '2016-01-01',
        },
      }),
    ).toEqual({ areaHaMax: 5000, verticesMax: 200, periodDaysMax: 366, minDate: '2016-01-01' });
  });

  it('частичный набор → null (валидация по половине чисел не выполняется)', () => {
    expect(adaptLimits({ limits: { area_ha_max: 5000 } })).toBeNull();
    expect(adaptLimits({ limits: null })).toBeNull();
    expect(adaptLimits(null)).toBeNull();
    expect(adaptLimits(undefined)).toBeNull();
  });

  it('лимиты из тела 422 извлекаются', () => {
    expect(
      adaptLimitsFromErrorBody({
        code: 'area_too_large',
        limits: { area_ha_max: 10, vertices_max: 5, period_days_max: 30, min_date: '2016-01-01' },
      }),
    ).toEqual({ areaHaMax: 10, verticesMax: 5, periodDaysMax: 30, minDate: '2016-01-01' });
  });
});

describe('normalizeError', () => {
  it('RFC 7807 → title/detail, code из type', () => {
    const error = normalizeError(422, {
      type: 'https://api.example.com/errors/area_too_large',
      title: 'Слишком большой участок',
      detail: 'Площадь превышает лимит',
    });
    expect(error.status).toBe(422);
    expect(error.code).toBe('area_too_large');
    expect(error.title).toBe('Слишком большой участок');
    expect(error.detail).toBe('Площадь превышает лимит');
  });

  it('{code, message} → единый AppError', () => {
    const error = normalizeError(400, { code: 'bad_period', message: 'Некорректный период' });
    expect(error.code).toBe('bad_period');
    expect(error.title).toBe('Некорректный период');
  });

  it('{error} и произвольное тело не ломают парсинг', () => {
    expect(normalizeError(500, { error: 'boom' }).code).toBe('error');
    expect(normalizeError(500, 'oops').code).toBe('unknown');
  });

  it('429 без кода → queue_full (corrections §1)', () => {
    expect(normalizeError(429, {}).code).toBe('queue_full');
  });

  it('сетевая ошибка → status 0', () => {
    const error = normalizeNetworkError(new Error('offline'));
    expect(error.status).toBe(0);
    expect(error.code).toBe('network');
  });
});

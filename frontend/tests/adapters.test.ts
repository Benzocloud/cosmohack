import { adaptArea, adaptAreaList } from '@/api/adapters/areas';
import { adaptContours } from '@/api/adapters/contours';
import { adaptEvent, adaptEventList } from '@/api/adapters/events';
import { adaptJob } from '@/api/adapters/jobs';
import { adaptLimits, adaptLimitsFromErrorBody } from '@/api/adapters/limits';
import { adaptSeries, adaptSeriesPoint } from '@/api/adapters/series';
import { normalizeError, normalizeNetworkError } from '@/api/client';
import { describe, expect, it } from 'vitest';

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

describe('canonical adapters', () => {
  it('maps contours and preserves provider metadata', () => {
    const result = adaptContours({
      contours: [
        {
          id: 'way/1',
          name: 'Field',
          geometry: polygon,
          source: { provider: 'overpass', attribution: 'OSM' },
        },
      ],
    });
    expect(result.status).toBe('ok');
    expect(result.contours[0]).toMatchObject({
      id: 'way/1',
      provider: 'overpass',
      attribution: 'OSM',
    });
    expect(adaptContours({ contours: [] }).status).toBe('empty');
  });
  it('maps area shown_result and active_job', () => {
    const area = adaptArea({
      id: 'a1',
      name: 'Field',
      geometry: polygon,
      source: { kind: 'contour', contour_id: 'way/1', provider: 'overpass' },
      period: { from: '2024-01-01', to: '2024-02-01' },
      created_at: '2026-01-01T00:00:00Z',
      shown_result: {
        result_version: 'v1',
        period: { from: '2024-01-01', to: '2024-02-01' },
        computed_at: '2026-01-01T00:00:00Z',
        status: 'normal',
        severity: null,
      },
      active_job: { job_id: 'j1', status: 'running', stage: 'analyze' },
    });
    expect(area.source.externalId).toBe('way/1');
    expect(area.lastResult?.resultVersion).toBe('v1');
    expect(area.activeJob?.id).toBe('j1');
    expect(
      adaptAreaList({
        areas: [
          {
            id: 'a2',
            name: 'Second field',
            geometry: polygon,
            source: { kind: 'drawn' },
            period: { from: '2024-01-01', to: '2024-02-01' },
            created_at: '2026-01-01T00:00:00Z',
            shown_result: null,
            active_job: null,
          },
        ],
      }),
    ).toHaveLength(1);
  });
  it('maps canonical job period and nested error', () => {
    const job = adaptJob({
      id: 'j1',
      area_id: 'a1',
      period: { from: '2024-01-01', to: '2024-02-01' },
      status: 'failed',
      stage: null,
      error: { code: 'source_unavailable', message: 'provider unavailable' },
      updated_at: '2026-01-01T00:00:00Z',
    });
    expect(job.errorCode).toBe('source_unavailable');
  });
  it('maps public series and empty result', () => {
    const series = adaptSeries({
      area_id: 'a1',
      result_version: 'v1',
      period: { from: '2024-01-01', to: '2024-02-01' },
      series: [
        {
          date: '2024-01-01',
          primary_ndvi: null,
          value: 0.4,
          state: 'imputed',
          method: 'linear interpolation',
          z_score: -2,
        },
      ],
      weather: [{ date: '2024-01-01', temperature_mean_c: 22, precipitation_sum_mm: 4 }],
    });
    expect(series.points[0]).toMatchObject({
      ndvi: 0.4,
      provenance: 'imputed',
      quality: 'linear interpolation',
      z: -2,
    });
    expect(series.weather?.temperature[0].value).toBe(22);
    expect(adaptSeries({ area_id: 'a1', series: [], weather: [] }).points).toEqual([]);
    expect(
      adaptSeries({
        area_id: 'a1',
        series: [{ date: '2024-01-01', value: 0.2 }],
        weather: [{ date: '2024-01-01', temperature_mean_c: null, precipitation_sum_mm: null }],
      }).weather,
    ).toBeNull();
    expect(adaptSeriesPoint({ date: 'd', value: null, state: 'missing' }).ndvi).toBeNull();
    expect(() => adaptSeriesPoint({ date: 'd', value: 0.2, state: 'invalid' as never })).toThrow(
      'unknown point state',
    );
  });
  it('rejects malformed area geometry', () => {
    expect(() =>
      adaptArea({
        id: 'a1',
        name: 'Field',
        geometry: { type: 'Polygon', coordinates: [[[Number.NaN, 1]]] },
        source: { kind: 'drawn' },
        created_at: '2026-01-01T00:00:00Z',
      }),
    ).toThrow('coordinates');
  });
  it('maps canonical anomaly events with indexed ids', () => {
    const event = adaptEvent(
      {
        start_date: '2024-06-01',
        end_date: '2024-06-10',
        status: 'confirmed',
        severity: 'high',
        min_z_score: -2.1,
        evidence_dates: ['2024-06-05'],
        facts: ['dry'],
        hypothesis: null,
        limitations: [],
      },
      3,
    );
    expect(event.id).toBe('2024-06-01:2024-06-10:3');
    expect(adaptEventList({ events: [] })).toEqual([]);
  });
  it('maps canonical config and nested validation limits', () => {
    expect(
      adaptLimits({
        area_ha_max: 5000,
        vertices_max: 200,
        period_days_max: 366,
        min_date: '2016-01-01',
      }),
    ).toEqual({ areaHaMax: 5000, verticesMax: 200, periodDaysMax: 366, minDate: '2016-01-01' });
    expect(
      adaptLimitsFromErrorBody({ limits: { area_ha_max: 10, vertices_max: 5 } }),
    ).toMatchObject({ areaHaMax: 10, verticesMax: 5 });
  });
  it('normalizes nested backend errors and network errors', () => {
    expect(
      normalizeError(503, { error: { code: 'source_unavailable', message: 'down' } }).code,
    ).toBe('source_unavailable');
    expect(normalizeNetworkError(new Error('offline')).status).toBe(0);
  });
});

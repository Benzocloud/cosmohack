import type { Provenance, Series, SeriesPoint } from '@/api/types';

/**
 * Адаптер ряда NDVI: GET /api/areas/{id}/series (frontend-plan §4).
 * Ключевые правила (design-brief §4, corrections §5):
 *  - null NDVI никогда не превращается в 0;
 *  - provenance: если backend не передал — выводим из данных
 *    (null → 'missing', число → 'observed'), переданный валидный — сохраняем;
 *  - отсутствующие необязательные поля → undefined/null, не выдуманные значения.
 */

export interface PointRaw {
  [key: string]: unknown;
  date?: string;
  primary_ndvi?: number | null;
  value?: number | null;
  state?: 'observed' | 'imputed' | 'missing';
  method?: string | null;
  z_score?: number | null;
  baseline?: number | null;
  ndvi?: number | null;
  provenance?: string | null;
  z?: number | null;
}

export interface SeriesRaw {
  [key: string]: unknown;
  area_id?: string;
  result_version?: string;
  period?: { from?: string; to?: string };
  weather?: unknown;
  series?: PointRaw[];
  points?: PointRaw[];
  schema_version?: string;
  feature_profile?: string;
  model_version?: string;
  method?: string;
  status?: string;
  limitations?: string[];
  usable_count?: number;
  imputed_count?: number;
  missing_count?: number;
}

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptSeries: required field ${field} is missing or empty`);
  }
  return value;
}

const PROVENANCES: Provenance[] = ['observed', 'imputed', 'missing'];
const RESULT_STATUSES = ['normal', 'candidate', 'confirmed', 'insufficient_data'] as const;

function adaptSeriesStatus(value?: string): Series['status'] {
  if (value === undefined) return undefined;
  const status = RESULT_STATUSES.find((candidate) => candidate === value);
  if (!status) throw new Error(`adaptSeries: unknown status ${value}`);
  return status;
}

export function adaptSeriesPoint(raw: PointRaw): SeriesPoint {
  // null/undefined/NaN NDVI → null («Нет данных»); число → число. Ноль — только настоящий 0.
  const rawValue = raw.value;
  const ndvi = typeof rawValue === 'number' && Number.isFinite(rawValue) ? rawValue : null;

  const provenance = raw.state
    ? PROVENANCES.find((value) => value === raw.state)
    : ndvi === null
      ? 'missing'
      : 'observed';
  if (!provenance) {
    throw new Error(`adaptSeries: unknown point state ${String(raw.state)}`);
  }

  return {
    date: requireString(raw.date, 'point.date'),
    ndvi,
    provenance,
    quality: raw.method ?? null,
    background: typeof raw.baseline === 'number' ? { mean: raw.baseline } : null,
    deviation: null,
    z: typeof raw.z_score === 'number' ? raw.z_score : null,
    source: null,
  };
}

export function adaptSeries(raw: SeriesRaw): Series {
  const weatherInput = raw.weather as
    | Array<{
        date?: string;
        temperature_mean_c?: number | null;
        precipitation_sum_mm?: number | null;
      }>
    | null
    | undefined;
  const weatherRaw = weatherInput
    ? {
        temperature: weatherInput.map((p) => ({ date: p.date, value: p.temperature_mean_c })),
        precipitation: weatherInput.map((p) => ({ date: p.date, value: p.precipitation_sum_mm })),
      }
    : null;
  const hasWeather =
    weatherInput?.some(
      (point) =>
        (typeof point.temperature_mean_c === 'number' &&
          Number.isFinite(point.temperature_mean_c)) ||
        (typeof point.precipitation_sum_mm === 'number' &&
          Number.isFinite(point.precipitation_sum_mm)),
    ) ?? false;
  const weather =
    weatherRaw && hasWeather
      ? {
          temperature: weatherRaw.temperature.map((p) => ({
            date: requireString(p.date, 'weather.date'),
            value: typeof p.value === 'number' ? p.value : null,
          })),
          precipitation: weatherRaw.precipitation.map((p) => ({
            date: requireString(p.date, 'weather.date'),
            value: typeof p.value === 'number' ? p.value : null,
          })),
          units: { temperature: '°C' as const, precipitation: 'мм' as const },
          aggregation: { temperature: '', precipitation: '' },
          coverage: { from: raw.period?.from ?? '', to: raw.period?.to ?? '' },
          source: '',
          spatialNote: '',
        }
      : null;

  return {
    areaId: requireString(raw.area_id, 'area_id'),
    resultVersion: raw.result_version ?? '',
    period: {
      from: raw.period?.from ?? '',
      to: raw.period?.to ?? '',
    },
    points: (raw.series ?? []).map(adaptSeriesPoint),
    background: null,
    weather,
    schemaVersion: raw.schema_version,
    featureProfile: raw.feature_profile,
    modelVersion: raw.model_version,
    method: raw.method,
    status: adaptSeriesStatus(raw.status),
    limitations: raw.limitations ?? [],
    usableCount: raw.usable_count ?? 0,
    imputedCount: raw.imputed_count ?? 0,
    missingCount: raw.missing_count ?? 0,
  };
}

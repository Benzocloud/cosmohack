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
  date?: string;
  ndvi?: number | null;
  provenance?: string | null;
  quality?: string | null;
  background?: { mean?: number | null; low?: number | null; high?: number | null } | null;
  deviation?: { value?: number | null; unit?: string; base?: string } | null;
  z?: number | null;
  source?: string | null;
}

export interface SeriesRaw {
  area_id?: string;
  result_version?: string;
  period?: { from?: string; to?: string };
  points?: PointRaw[];
  background?: {
    label?: string;
    years_from?: number;
    years_to?: number;
    source?: string;
    band_meaning?: string;
  } | null;
  weather?: {
    temperature?: { date?: string; value?: number | null }[];
    precipitation?: { date?: string; value?: number | null }[];
    units?: { temperature?: string; precipitation?: string };
    aggregation?: { temperature?: string; precipitation?: string };
    coverage?: { from?: string; to?: string };
    source?: string;
    spatial_note?: string;
  } | null;
}

const PROVENANCES: Provenance[] = ['observed', 'imputed', 'missing'];
const DEVIATION_UNITS = ['ndvi', 'percent'] as const;

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptSeries: обязательное поле ${field} отсутствует или пустое`);
  }
  return value;
}

export function adaptSeriesPoint(raw: PointRaw): SeriesPoint {
  // null/undefined/NaN NDVI → null («Нет данных»); число → число. Ноль — только настоящий 0.
  const ndvi = typeof raw.ndvi === 'number' && Number.isFinite(raw.ndvi) ? raw.ndvi : null;

  const declared = PROVENANCES.find((p) => p === raw.provenance);
  // provenance по данным, если backend его не передал
  const provenance: Provenance = declared ?? (ndvi === null ? 'missing' : 'observed');

  let deviation: SeriesPoint['deviation'] = null;
  if (raw.deviation && typeof raw.deviation.value === 'number') {
    const unit = DEVIATION_UNITS.find((u) => u === raw.deviation?.unit);
    // неизвестная единица → отклонение не показываем (не выдумываем интерпретацию)
    if (unit) {
      deviation = { value: raw.deviation.value as number, unit, base: raw.deviation.base ?? '' };
    }
  }

  const bg = raw.background;
  const background =
    bg && typeof bg.mean === 'number'
      ? {
          mean: bg.mean,
          low: typeof bg.low === 'number' ? bg.low : undefined,
          high: typeof bg.high === 'number' ? bg.high : undefined,
        }
      : null;

  return {
    date: requireString(raw.date, 'point.date'),
    ndvi,
    provenance,
    quality: raw.quality ?? null,
    background,
    deviation,
    z: typeof raw.z === 'number' ? raw.z : null,
    source: raw.source ?? null,
  };
}

export function adaptSeries(raw: SeriesRaw): Series {
  const weather = raw.weather
    ? {
        temperature: (raw.weather.temperature ?? []).map((p) => ({
          date: requireString(p.date, 'weather.temperature.date'),
          value: typeof p.value === 'number' ? p.value : null,
        })),
        precipitation: (raw.weather.precipitation ?? []).map((p) => ({
          date: requireString(p.date, 'weather.precipitation.date'),
          value: typeof p.value === 'number' ? p.value : null,
        })),
        units: {
          temperature: (raw.weather.units?.temperature ?? '°C') as '°C',
          precipitation: (raw.weather.units?.precipitation ?? 'мм') as 'мм',
        },
        aggregation: {
          temperature: raw.weather.aggregation?.temperature ?? '',
          precipitation: raw.weather.aggregation?.precipitation ?? '',
        },
        coverage: {
          from: requireString(raw.weather.coverage?.from, 'weather.coverage.from'),
          to: requireString(raw.weather.coverage?.to, 'weather.coverage.to'),
        },
        source: raw.weather.source ?? '',
        spatialNote: raw.weather.spatial_note ?? '',
      }
    : null;

  return {
    areaId: requireString(raw.area_id, 'area_id'),
    resultVersion: requireString(raw.result_version, 'result_version'),
    period: {
      from: requireString(raw.period?.from, 'period.from'),
      to: requireString(raw.period?.to, 'period.to'),
    },
    points: (raw.points ?? []).map(adaptSeriesPoint),
    background: raw.background
      ? {
          label: raw.background.label ?? '',
          yearsFrom: typeof raw.background.years_from === 'number' ? raw.background.years_from : 0,
          yearsTo: typeof raw.background.years_to === 'number' ? raw.background.years_to : 0,
          source: raw.background.source ?? '',
          bandMeaning: raw.background.band_meaning,
        }
      : null,
    weather,
  };
}

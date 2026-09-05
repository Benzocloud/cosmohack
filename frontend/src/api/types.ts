/**
 * View-модели интерфейса (frontend-plan §4) — НЕ OpenAPI-схема.
 * Компоненты работают только с этими типами; когда B3 меняет поле,
 * правится один адаптер (этап FE-1).
 */

export type JobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
export type Verdict = 'normal' | 'candidate' | 'confirmed' | 'insufficient_data';
export type Provenance = 'observed' | 'imputed' | 'missing';
export type SourceStatus = 'ok' | 'partial' | 'unavailable';

export interface Period {
  from: string;
  to: string;
} // YYYY-MM-DD, UTC

export interface Area {
  id: string;
  name: string;
  geometry: GeoJSON.Polygon;
  source: { kind: 'contour' | 'drawn'; label: string; externalId?: string; cropType?: string }; // label: «Контур OpenStreetMap», «Нарисован вручную»
  period?: Period;
  createdAt: string;
  lastResult?: ResultMeta; // сохранённый результат (может быть от старого периода)
  activeJob?: JobMeta; // текущая задача, если есть
}

export interface ResultMeta {
  resultVersion: string;
  period: Period;
  computedAt: string;
  verdict: Verdict;
  severity?: string | null; // severity — строка от ML, показываем как есть
  modelVersion?: string;
  method?: string;
  usableCount?: number;
  imputedCount?: number;
  missingCount?: number;
  sources: Record<string, { status: SourceStatus; note?: string }>; // {'sentinel2': {...}, 'modis': ..., 'era5': ...}
  limitations: string[];
}

export interface JobMeta {
  id: string;
  areaId: string;
  requestedPeriod: Period;
  status: JobStatus;
  stage?: string | null;
  message?: string | null;
  errorCode?: string | null;
  errorMessage?: string | null;
  resultVersion?: string | null;
  updatedAt: string;
}

export interface SeriesPoint {
  date: string;
  ndvi: number | null;
  provenance: Provenance;
  quality?: string | null; // текстовое качество от B1, если есть
  background?: { mean: number; low?: number; high?: number } | null;
  deviation?: { value: number; unit: 'ndvi' | 'percent'; base: string } | null; // «−0.18 NDVI к сезонному фону 2017–2024»
  z?: number | null;
  source?: string | null;
}

export interface Series {
  areaId: string;
  resultVersion: string;
  period: Period;
  points: SeriesPoint[];
  background?: {
    label: string;
    yearsFrom: number;
    yearsTo: number;
    source: string;
    bandMeaning?: string;
  } | null;
  weather?: {
    temperature: { date: string; value: number | null }[];
    precipitation: { date: string; value: number | null }[];
    units: { temperature: '°C'; precipitation: 'мм' };
    aggregation: { temperature: string; precipitation: string }; // «средняя за интервал», «сумма за интервал»
    coverage: Period;
    source: string;
    spatialNote: string; // «ячейка реанализа ERA5-Land ~9 км»
  } | null;
  schemaVersion?: string;
  featureProfile?: string;
  modelVersion?: string;
  method?: string;
  status?: Verdict;
  limitations: string[];
  usableCount: number;
  imputedCount: number;
  missingCount: number;
}

export interface AnalysisEvent {
  id: string;
  period: Period;
  verdict: 'candidate' | 'confirmed';
  severity?: string | null;
  detected: { magnitude: number | null; unit: 'ndvi' | 'percent'; base: string; text: string };
  basis: {
    observedCount: number;
    imputedCount: number;
    backgroundComparable: boolean;
    gapsNote?: string;
    criteria?: string;
  };
  weather?: { facts: { label: string; value: string }[]; hypothesis?: string | null } | null;
  limitations: string[];
}

export interface ResultBundle {
  meta: ResultMeta;
  series: Series;
  events: AnalysisEvent[];
} // одна версия целиком

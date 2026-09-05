import type { Area, ResultMeta, SourceStatus, Verdict } from '@/api/types';
import { adaptActiveJob } from './jobs';

/**
 * Адаптеры участка и метаданных результата (frontend-plan §4, адаптеры таблицы §6).
 * Отсутствующие необязательные поля → undefined; известные null → null.
 * Верификаты/статусы вне контракта считаются ошибкой адаптера (не подменяем).
 */

export interface ResultMetaRaw {
  verdict?: string;
  result_version?: string;
  period?: { from?: string; to?: string };
  computed_at?: string;
  status?: string;
  severity?: string | null;
  sources?: Record<string, { status?: string; note?: string } | null>;
  limitations?: string[];
}

export interface ShownResultRaw {
  result_version?: string;
  job_id?: string;
  period?: { from?: string; to?: string };
  computed_at?: string;
  status?: string;
  severity?: string | null;
}

export interface AreaRaw {
  [key: string]: unknown;
  id?: string;
  name?: string;
  geometry?: GeoJSON.Polygon;
  source?: {
    kind?: string;
    label?: string;
    external_id?: string;
    contour_id?: string;
    provider?: string;
  };
  period?: { from?: string; to?: string };
  created_at?: string;
  shown_result?: ShownResultRaw | null;
  last_result?: ResultMetaRaw | null;
  active_job?: { job_id?: string; status?: string; stage?: string | null } | null;
}

const VERDICTS: Verdict[] = ['normal', 'candidate', 'confirmed', 'insufficient_data'];
const SOURCE_STATUSES: SourceStatus[] = ['ok', 'partial', 'unavailable'];
const SOURCE_KINDS = ['contour', 'drawn'] as const;

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptArea: required field ${field} is missing or empty`);
  }
  return value;
}

function requirePolygon(value: unknown): GeoJSON.Polygon {
  if (
    typeof value !== 'object' ||
    value === null ||
    (value as { type?: unknown }).type !== 'Polygon' ||
    !Array.isArray((value as { coordinates?: unknown }).coordinates)
  ) {
    throw new Error('adaptArea: geometry must be a GeoJSON Polygon');
  }
  const coordinates = (value as { coordinates: unknown[] }).coordinates;
  if (coordinates.length === 0 || coordinates.some((ring) => !Array.isArray(ring))) {
    throw new Error('adaptArea: polygon coordinates are invalid');
  }
  for (const ring of coordinates as unknown[]) {
    for (const point of ring as unknown[]) {
      if (
        !Array.isArray(point) ||
        point.length < 2 ||
        typeof point[0] !== 'number' ||
        typeof point[1] !== 'number' ||
        !Number.isFinite(point[0]) ||
        !Number.isFinite(point[1])
      ) {
        throw new Error('adaptArea: polygon coordinates are invalid');
      }
    }
  }
  return value as GeoJSON.Polygon;
}

export function adaptResultMeta(raw: ResultMetaRaw): ResultMeta {
  const verdict = VERDICTS.find((v) => v === raw.status);
  if (!verdict) {
    throw new Error(`adaptResultMeta: unknown status ${String(raw.status)}`);
  }

  const sources: ResultMeta['sources'] = {};
  for (const [key, value] of Object.entries(raw.sources ?? {})) {
    if (!value) continue;
    const status = SOURCE_STATUSES.find((s) => s === value.status);
    if (!status) {
      throw new Error(`adaptResultMeta: unknown source status ${String(value.status)} (${key})`);
    }
    sources[key] = { status, note: value.note };
  }

  return {
    resultVersion: requireString(raw.result_version, 'last_result.result_version'),
    period: {
      from: requireString(raw.period?.from, 'period.from'),
      to: requireString(raw.period?.to, 'period.to'),
    },
    computedAt: requireString(raw.computed_at, 'last_result.computed_at'),
    verdict,
    severity: raw.severity ?? null,
    sources,
    limitations: raw.limitations ?? [],
  };
}

export function adaptArea(raw: AreaRaw): Area {
  const kindRaw = raw.source?.kind;
  const kind = SOURCE_KINDS.find((k) => k === kindRaw);
  if (!kind) {
    throw new Error(`adaptArea: unknown source.kind ${String(kindRaw)}`);
  }

  const shown = raw.shown_result;
  const sourceLabel =
    raw.source?.label ??
    (raw.source?.provider
      ? `Контур ${raw.source.provider}`
      : kind === 'drawn'
        ? 'Нарисован вручную'
        : 'Контур');
  return {
    id: requireString(raw.id, 'id'),
    name: requireString(raw.name, 'name'),
    geometry: requirePolygon(raw.geometry),
    source: {
      kind,
      label: sourceLabel,
      externalId: raw.source?.external_id ?? raw.source?.contour_id,
    },
    ...(raw.period?.from && raw.period?.to
      ? { period: { from: raw.period.from, to: raw.period.to } }
      : {}),
    createdAt: requireString(raw.created_at, 'created_at'),
    lastResult: shown ? adaptResultMeta(shown) : undefined,
    activeJob:
      raw.active_job && raw.id
        ? adaptActiveJob(raw.active_job, raw.id, raw.period ?? {})
        : undefined,
  };
}

export function adaptAreaList(raw: unknown): Area[] {
  const list = Array.isArray(raw)
    ? raw
    : typeof raw === 'object' && raw !== null && 'areas' in raw
      ? (raw as { areas: unknown[] }).areas
      : null;
  if (!Array.isArray(list)) {
    throw new Error('adaptAreaList: expected an array or {areas: [...]}');
  }
  return list.map((item) => adaptArea(item as AreaRaw));
}

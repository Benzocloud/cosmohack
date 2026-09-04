import type { Area, ResultMeta, SourceStatus, Verdict } from '@/api/types';
import { type JobRaw, adaptJob } from './jobs';

/**
 * Адаптеры участка и метаданных результата (frontend-plan §4, адаптеры таблицы §6).
 * Отсутствующие необязательные поля → undefined; известные null → null.
 * Верификаты/статусы вне контракта считаются ошибкой адаптера (не подменяем).
 */

export interface ResultMetaRaw {
  result_version?: string;
  period?: { from?: string; to?: string };
  computed_at?: string;
  verdict?: string;
  severity?: string | null;
  sources?: Record<string, { status?: string; note?: string } | null>;
  limitations?: string[];
}

export interface AreaRaw {
  id?: string;
  name?: string;
  geometry?: GeoJSON.Polygon | GeoJSON.MultiPolygon;
  source?: { kind?: string; label?: string; external_id?: string };
  created_at?: string;
  last_result?: ResultMetaRaw | null;
  active_job?: JobRaw | null;
}

const VERDICTS: Verdict[] = ['normal', 'candidate', 'confirmed', 'insufficient_data'];
const SOURCE_STATUSES: SourceStatus[] = ['ok', 'partial', 'unavailable'];
const SOURCE_KINDS = ['contour', 'drawn'] as const;

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptArea: обязательное поле ${field} отсутствует или пустое`);
  }
  return value;
}

export function adaptResultMeta(raw: ResultMetaRaw): ResultMeta {
  const verdict = VERDICTS.find((v) => v === raw.verdict);
  if (!verdict) {
    throw new Error(`adaptResultMeta: неизвестный verdict «${String(raw.verdict)}»`);
  }

  const sources: ResultMeta['sources'] = {};
  for (const [key, value] of Object.entries(raw.sources ?? {})) {
    if (!value) continue;
    const status = SOURCE_STATUSES.find((s) => s === value.status);
    if (!status) {
      throw new Error(
        `adaptResultMeta: неизвестный статус источника «${String(value.status)}» (${key})`,
      );
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
    throw new Error(`adaptArea: неизвестный source.kind «${String(kindRaw)}»`);
  }

  return {
    id: requireString(raw.id, 'id'),
    name: requireString(raw.name, 'name'),
    geometry: raw.geometry as GeoJSON.Polygon | GeoJSON.MultiPolygon,
    source: {
      kind,
      label: requireString(raw.source?.label, 'source.label'),
      externalId: raw.source?.external_id,
    },
    createdAt: requireString(raw.created_at, 'created_at'),
    lastResult: raw.last_result ? adaptResultMeta(raw.last_result) : undefined,
    activeJob: raw.active_job ? adaptJob(raw.active_job) : undefined,
  };
}

export function adaptAreaList(raw: unknown): Area[] {
  const list = Array.isArray(raw)
    ? raw
    : typeof raw === 'object' && raw !== null && 'areas' in raw
      ? (raw as { areas: unknown[] }).areas
      : null;
  if (!Array.isArray(list)) {
    throw new Error('adaptAreaList: ожидается массив или {areas: [...]}');
  }
  return list.map((item) => adaptArea(item as AreaRaw));
}

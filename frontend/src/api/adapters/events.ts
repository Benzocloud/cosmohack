import type { AnalysisEvent } from '@/api/types';

/**
 * Адаптер событий: GET /api/areas/{id}/events (frontend-plan §4).
 * Четыре блока карточки (обнаружено/основание/погода/ограничения) приходят готовыми:
 * frontend ничего не вычисляет — только маппит поля.
 */

export interface EventRaw {
  [key: string]: unknown;
  start_date?: string;
  end_date?: string;
  status?: string;
  severity?: string | null;
  min_z_score?: number | null;
  evidence_dates?: string[];
  facts?: string[];
  hypothesis?: string | null;
  limitations?: string[];
  verdict?: string;
  period?: { from?: string; to?: string };
  detected?: unknown;
  weather?: unknown;
}

const EVENT_VERDICTS = ['candidate', 'confirmed'] as const;
function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptEvent: required field ${field} is missing or empty`);
  }
  return value;
}

export function adaptEvent(raw: EventRaw, index = 0): AnalysisEvent {
  const verdict = EVENT_VERDICTS.find((v) => v === raw.status);
  if (!verdict) {
    throw new Error(`adaptEvent: unknown event status ${String(raw.status)}`);
  }

  const weather =
    raw.facts?.length || raw.hypothesis
      ? {
          facts: (raw.facts ?? []).map((fact) => ({ label: 'Факт', value: fact })),
          hypothesis: raw.hypothesis ?? null,
        }
      : null;

  return {
    id: `${requireString(raw.start_date, 'start_date')}:${requireString(raw.end_date, 'end_date')}:${index}`,
    period: {
      from: requireString(raw.start_date, 'start_date'),
      to: requireString(raw.end_date, 'end_date'),
    },
    verdict,
    severity: raw.severity ?? null,
    detected: {
      magnitude: null,
      unit: 'ndvi',
      base: 'z-score',
      text: raw.min_z_score == null ? '' : `min z-score ${raw.min_z_score}`,
    },
    basis: {
      observedCount: raw.evidence_dates?.length ?? 0,
      imputedCount: 0,
      backgroundComparable: true,
    },
    weather,
    limitations: raw.limitations ?? [],
  };
}

export function adaptEventList(raw: unknown): AnalysisEvent[] {
  const list = Array.isArray(raw)
    ? raw
    : typeof raw === 'object' && raw !== null && 'events' in raw
      ? (raw as { events: unknown[] }).events
      : null;
  if (!Array.isArray(list)) {
    throw new Error('adaptEventList: expected an array or {events: [...]}');
  }
  return list.map((item, index) => adaptEvent(item as EventRaw, index));
}

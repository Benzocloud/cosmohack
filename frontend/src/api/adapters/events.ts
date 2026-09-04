import type { AnalysisEvent } from '@/api/types';

/**
 * Адаптер событий: GET /api/areas/{id}/events (frontend-plan §4).
 * Четыре блока карточки (обнаружено/основание/погода/ограничения) приходят готовыми:
 * frontend ничего не вычисляет — только маппит поля.
 */

export interface EventRaw {
  id?: string;
  period?: { from?: string; to?: string };
  verdict?: string;
  severity?: string | null;
  detected?: { magnitude?: number | null; unit?: string; base?: string; text?: string };
  basis?: {
    observed_count?: number;
    imputed_count?: number;
    background_comparable?: boolean;
    gaps_note?: string;
    criteria?: string;
  };
  weather?: { facts?: { label?: string; value?: string }[]; hypothesis?: string | null } | null;
  limitations?: string[];
}

const EVENT_VERDICTS = ['candidate', 'confirmed'] as const;
const DETECTED_UNITS = ['ndvi', 'percent'] as const;

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptEvent: обязательное поле ${field} отсутствует или пустое`);
  }
  return value;
}

export function adaptEvent(raw: EventRaw): AnalysisEvent {
  const verdict = EVENT_VERDICTS.find((v) => v === raw.verdict);
  if (!verdict) {
    throw new Error(`adaptEvent: неизвестный verdict события «${String(raw.verdict)}»`);
  }

  const detectedRaw = raw.detected ?? {};
  const unit = DETECTED_UNITS.find((u) => u === detectedRaw.unit);
  if (!unit || typeof detectedRaw.magnitude !== 'number') {
    throw new Error('adaptEvent: некорректное поле detected (magnitude/unit)');
  }

  const weather = raw.weather
    ? {
        facts: (raw.weather.facts ?? []).map((fact) => ({
          label: requireString(fact.label, 'weather.facts.label'),
          value: requireString(fact.value, 'weather.facts.value'),
        })),
        // hypothesis: null остаётся null → UI покажет «Причина по доступным данным не установлена»
        hypothesis: raw.weather.hypothesis ?? null,
      }
    : undefined;

  return {
    id: requireString(raw.id, 'id'),
    period: {
      from: requireString(raw.period?.from, 'period.from'),
      to: requireString(raw.period?.to, 'period.to'),
    },
    verdict,
    severity: raw.severity ?? null,
    detected: {
      magnitude: detectedRaw.magnitude,
      unit,
      base: detectedRaw.base ?? '',
      text: detectedRaw.text ?? '',
    },
    basis: {
      observedCount: typeof raw.basis?.observed_count === 'number' ? raw.basis.observed_count : 0,
      imputedCount: typeof raw.basis?.imputed_count === 'number' ? raw.basis.imputed_count : 0,
      backgroundComparable: raw.basis?.background_comparable === true,
      gapsNote: raw.basis?.gaps_note,
      criteria: raw.basis?.criteria,
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
    throw new Error('adaptEventList: ожидается массив или {events: [...]}');
  }
  return list.map((item) => adaptEvent(item as EventRaw));
}

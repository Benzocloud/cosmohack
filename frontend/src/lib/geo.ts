import { area as turfArea } from '@turf/area';
import { kinks } from '@turf/kinks';

import type { Limits } from '@/api/adapters/limits';
import { MAP_LABELS } from '@/lib/labels';

/**
 * Клиентская геометрия полигона (frontend-plan §8, lib/geo).
 * corrections §1: самопересечение и число вершин проверяются ВСЕГДА;
 * площадь и остальные лимиты — только если useLimits вернул не null.
 * Числа лимитов приходят из бэкенда — в коде их нет.
 * Серверная проверка остаётся окончательной (бриф §3A).
 */

export type Ring = [number, number][];

/** Замкнутый GeoJSON-полигон из незамкнутого набора вершин. */
export function toPolygonGeometry(ring: Ring): GeoJSON.Polygon {
  const first = ring[0];
  const last = ring[ring.length - 1];
  const closed =
    first && last && first[0] === last[0] && first[1] === last[1] ? ring : [...ring, first];
  return { type: 'Polygon', coordinates: [closed] };
}

/** Закрывает внешнее кольцо перед отправкой GeoJSON на сервер. */
export function closePolygonGeometry(geometry: GeoJSON.Polygon): GeoJSON.Polygon {
  const rings = geometry.coordinates.map((ring, index) => {
    if (index !== 0 || ring.length === 0) return ring;
    const first = ring[0];
    const last = ring[ring.length - 1];
    if (first[0] === last[0] && first[1] === last[1]) return ring;
    return [...ring, first];
  });
  return { ...geometry, coordinates: rings };
}

export function isSelfIntersecting(ring: Ring): boolean {
  if (ring.length < 4) return false;
  return kinks(toPolygonGeometry(ring)).features.length > 0;
}

export function polygonAreaHa(ring: Ring): number {
  if (ring.length < 3) return 0;
  return turfArea(toPolygonGeometry(ring)) / 10_000;
}

export interface PolygonValidation {
  ok: boolean;
  error?: string;
}

/** Самопересечение и минимум вершин — всегда; лимиты — только при известном limits. */
export function validatePolygon(ring: Ring, limits: Limits | null): PolygonValidation {
  if (ring.length < 3) {
    return { ok: false, error: MAP_LABELS.tooFewVertices };
  }
  if (isSelfIntersecting(ring)) {
    return { ok: false, error: MAP_LABELS.selfIntersection };
  }
  if (limits && ring.length > limits.verticesMax) {
    return { ok: false, error: MAP_LABELS.verticesLimit(limits.verticesMax) };
  }
  if (limits && polygonAreaHa(ring) > limits.areaHaMax) {
    return { ok: false, error: MAP_LABELS.areaLimit(limits.areaHaMax) };
  }
  return { ok: true };
}

export interface PeriodValidation {
  ok: boolean;
  error?: string;
}

/** Период: from ≤ to всегда; ограничение длины — только при известном лимите. */
export function validatePeriod(from: string, to: string, limits: Limits | null): PeriodValidation {
  if (!from || !to || from > to) {
    return { ok: false, error: MAP_LABELS.badPeriod };
  }
  if (limits?.periodDaysMax !== undefined) {
    const days = (Date.parse(to) - Date.parse(from)) / 86_400_000 + 1;
    if (days > limits.periodDaysMax) {
      return { ok: false, error: MAP_LABELS.periodLimit(limits.periodDaysMax) };
    }
  }
  return { ok: true };
}

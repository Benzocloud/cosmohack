/**
 * Адаптер контуров: GET /api/regions/contours?bbox=…
 * Ключевое требование: различать empty (контуров нет — это не ошибка)
 * и failed (источник не ответил) — design-brief §3A, corrections §1.
 */

export interface Contour {
  id: string;
  name?: string;
  externalId?: string;
  provider?: string;
  attribution?: string;
  geometry: GeoJSON.Polygon;
}

export type ContoursStatus = 'ok' | 'empty' | 'failed';

export interface ContoursResult {
  status: ContoursStatus;
  source?: string;
  coverageNote?: string;
  contours: Contour[];
}

export interface ContoursRaw {
  [key: string]: unknown;
  contours?: Array<{
    id?: string;
    name?: string;
    geometry?: GeoJSON.Polygon;
    source?: { provider?: string; attribution?: string };
  }>;
  status?: string;
  source?: string;
  coverage_note?: string;
  features?: unknown[];
}

export function adaptContours(raw: ContoursRaw): ContoursResult {
  const actual = raw.contours ?? [];
  const status: ContoursStatus = actual.length > 0 ? 'ok' : 'empty';

  const contours: Contour[] = [];
  if (status === 'ok') {
    for (const item of actual) {
      if (!item.id || !item.geometry) continue;
      contours.push({
        id: item.id,
        name: item.name,
        externalId: item.id,
        provider: item.source?.provider,
        attribution: item.source?.attribution,
        geometry: item.geometry,
      });
    }
  }
  return {
    status,
    contours,
  };
}

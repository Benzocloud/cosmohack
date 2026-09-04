/**
 * Адаптер контуров: GET /api/regions/contours?bbox=…
 * Ключевое требование: различать empty (контуров нет — это не ошибка)
 * и failed (источник не ответил) — design-brief §3A, corrections §1.
 */

export interface Contour {
  id: string;
  name?: string;
  externalId?: string;
  geometry: GeoJSON.Polygon | GeoJSON.MultiPolygon;
}

export type ContoursStatus = 'ok' | 'empty' | 'failed';

export interface ContoursResult {
  status: ContoursStatus;
  source?: string;
  coverageNote?: string;
  contours: Contour[];
}

export interface ContoursRaw {
  status?: string;
  source?: string;
  coverage_note?: string;
  features?: Array<{
    geometry?: GeoJSON.Polygon | GeoJSON.MultiPolygon;
    properties?: { id?: string; name?: string; external_id?: string };
  }>;
}

export function adaptContours(raw: ContoursRaw): ContoursResult {
  const status: ContoursStatus =
    raw.status === 'ok' || raw.status === 'empty' || raw.status === 'failed'
      ? raw.status
      : 'failed';

  const contours: Contour[] = [];
  if (status === 'ok' && raw.features) {
    for (const feature of raw.features) {
      // Контур без id или геометрии — некорректная запись каталога, пропускаем (ничего не выдумываем)
      if (!feature.properties?.id || !feature.geometry) continue;
      contours.push({
        id: feature.properties.id,
        name: feature.properties.name,
        externalId: feature.properties.external_id,
        geometry: feature.geometry,
      });
    }
  }

  return {
    status,
    source: raw.source,
    coverageNote: raw.coverage_note,
    contours,
  };
}

/**
 * Адаптер лимитов формы (corrections §1): числа приходят ТОЛЬКО из ответа бэкенда —
 * GET /api/config {limits:{…}} или поле limits в теле 422. В коде ничего не хардкодится.
 * Результат null = лимиты не пришли → клиентская числовая валидация не выполняется.
 */

export interface Limits {
  areaHaMax: number;
  verticesMax: number;
  periodDaysMax?: number;
  minDate?: string;
}

export interface LimitsRaw {
  area_ha_max?: number;
  max_area_km2?: number;
  vertices_max?: number;
  max_vertices?: number;
  period_days_max?: number;
  min_date?: string;
  limits?: LimitsRaw | null;
}

function adaptLimitsObject(limits: LimitsRaw): Limits | null {
  const {
    area_ha_max: areaHaMax,
    max_area_km2: maxAreaKm2,
    vertices_max: verticesMaxLegacy,
    max_vertices: verticesMaxCanonical,
    period_days_max: periodDaysMax,
    min_date: minDate,
  } = limits;
  // Частичные лимиты не собираем: валидация по половине чисел хуже отсутствия валидации
  if (
    (typeof areaHaMax !== 'number' && typeof maxAreaKm2 !== 'number') ||
    (typeof verticesMaxLegacy !== 'number' && typeof verticesMaxCanonical !== 'number')
  ) {
    return null;
  }
  return {
    areaHaMax: typeof areaHaMax === 'number' ? areaHaMax : (maxAreaKm2 as number) * 100,
    verticesMax:
      typeof verticesMaxLegacy === 'number' ? verticesMaxLegacy : (verticesMaxCanonical as number),
    ...(typeof periodDaysMax === 'number' ? { periodDaysMax } : {}),
    ...(typeof minDate === 'string' && minDate.length > 0 ? { minDate } : {}),
  };
}

/** Из тела /api/config или поля limits в 422. */
export function adaptLimits(raw: LimitsRaw | null | undefined): Limits | null {
  if (!raw) return null;
  return adaptLimitsObject(raw.limits ?? raw);
}

/** Из AppError.extra тела 422 (extra — всё тело ошибки). */
export function adaptLimitsFromErrorBody(body: unknown): Limits | null {
  if (typeof body !== 'object' || body === null || !('limits' in body)) return null;
  return adaptLimits(body as LimitsRaw);
}

/**
 * Адаптер лимитов формы (corrections §1): числа приходят ТОЛЬКО из ответа бэкенда —
 * GET /api/config {limits:{…}} или поле limits в теле 422. В коде ничего не хардкодится.
 * Результат null = лимиты не пришли → клиентская числовая валидация не выполняется.
 */

export interface Limits {
  areaHaMax: number;
  verticesMax: number;
  periodDaysMax: number;
  minDate: string;
}

export interface LimitsRaw {
  limits?: {
    area_ha_max?: number;
    vertices_max?: number;
    period_days_max?: number;
    min_date?: string;
  } | null;
}

function adaptLimitsObject(limits: NonNullable<NonNullable<LimitsRaw['limits']>>): Limits | null {
  const {
    area_ha_max: areaHaMax,
    vertices_max: verticesMax,
    period_days_max: periodDaysMax,
    min_date: minDate,
  } = limits;
  // Частичные лимиты не собираем: валидация по половине чисел хуже отсутствия валидации
  if (
    typeof areaHaMax !== 'number' ||
    typeof verticesMax !== 'number' ||
    typeof periodDaysMax !== 'number' ||
    typeof minDate !== 'string' ||
    minDate.length === 0
  ) {
    return null;
  }
  return { areaHaMax, verticesMax, periodDaysMax, minDate };
}

/** Из тела /api/config или поля limits в 422. */
export function adaptLimits(raw: LimitsRaw | null | undefined): Limits | null {
  if (!raw || !raw.limits) return null;
  return adaptLimitsObject(raw.limits);
}

/** Из AppError.extra тела 422 (extra — всё тело ошибки). */
export function adaptLimitsFromErrorBody(body: unknown): Limits | null {
  if (typeof body !== 'object' || body === null || !('limits' in body)) return null;
  return adaptLimits(body as LimitsRaw);
}

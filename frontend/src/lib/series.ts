import type { SeriesPoint } from '@/api/types';

/**
 * Разбиение ряда по происхождению для графика NDVI (frontend-plan §7).
 *
 * Правила визуализации (design-brief §4):
 *  - наблюдения соединяются линией только между соседними наблюдениями;
 *  - восстановленные участки рисуются отдельной пунктирной серией, включающей
 *    граничные наблюдения (чтобы пунктир «висел» между двумя реальными точками);
 *  - точки provenance='missing' дают разрыв в обеих сериях (null + connectNulls:false).
 *
 * Функция ничего не вычисляет про анализ: только раскладка уже готовых значений.
 */

export type DateValue = [string, number | null];
export type DateNumber = [string, number];

export interface SplitSeries {
  observedLine: DateValue[];
  observedDots: DateNumber[];
  imputedLine: DateValue[];
  imputedDots: DateNumber[];
  bandLow: DateValue[];
  bandDelta: DateValue[];
  bgMean: DateValue[];
}

const withValue = (point: SeriesPoint): point is SeriesPoint & { ndvi: number } =>
  point.ndvi !== null && Number.isFinite(point.ndvi);

export function splitByProvenance(points: SeriesPoint[]): SplitSeries {
  const observedLine: DateValue[] = [];
  const observedDots: DateNumber[] = [];
  const imputedLine: DateValue[] = [];
  const imputedDots: DateNumber[] = [];
  const bandLow: DateValue[] = [];
  const bandDelta: DateValue[] = [];
  const bgMean: DateValue[] = [];

  // Наблюдения: значение только на наблюдаемых точках; остальное — разрыв
  for (const point of points) {
    const observedValue = point.provenance === 'observed' && withValue(point) ? point.ndvi : null;
    observedLine.push([point.date, observedValue]);
    if (observedValue !== null) {
      observedDots.push([point.date, observedValue]);
    }
  }

  // Восстановление: значения на imputed-точках + граничные наблюдения вокруг
  // каждой последовательной серии imputed (иначе пунктир не «привяжется» к данным).
  // Пропуск (missing/null) границы не даёт: пунктир обрывается.
  const isImputed = (index: number): boolean =>
    points[index] !== undefined && points[index].provenance === 'imputed';

  for (let index = 0; index < points.length; index += 1) {
    const point = points[index];
    const boundary =
      (isImputed(index - 1) || isImputed(index + 1)) &&
      point.provenance === 'observed' &&
      withValue(point);

    if (point.provenance === 'imputed' && withValue(point)) {
      imputedLine.push([point.date, point.ndvi]);
      imputedDots.push([point.date, point.ndvi]);
    } else if (boundary) {
      imputedLine.push([point.date, point.ndvi]);
    } else {
      imputedLine.push([point.date, null]);
    }
  }

  // Сезонный фон: только там, где он реально передан; отсутствие → разрыв, не ноль
  for (const point of points) {
    if (point.background && Number.isFinite(point.background.mean)) {
      const { mean, low, high } = point.background;
      bgMean.push([point.date, mean]);
      bandLow.push([point.date, typeof low === 'number' ? low : mean]);
      bandDelta.push([point.date, typeof high === 'number' ? high - (low ?? mean) : 0]);
    } else {
      bandLow.push([point.date, null]);
      bandDelta.push([point.date, null]);
      bgMean.push([point.date, null]);
    }
  }

  return { observedLine, observedDots, imputedLine, imputedDots, bandLow, bandDelta, bgMean };
}

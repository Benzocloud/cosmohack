import type { Provenance, SeriesPoint } from '@/api/types';
import { splitByProvenance } from '@/lib/series';
import { describe, expect, it } from 'vitest';

/**
 * Тесты splitByProvenance (frontend-plan §10 FE-4, перенесены на FE-1):
 * одиночный пропуск, серия imputed между наблюдениями, missing даёт разрыв
 * в обеих сериях, фон отсутствует.
 */

const point = (date: string, ndvi: number | null, provenance: Provenance): SeriesPoint => ({
  date,
  ndvi,
  provenance,
});

const dates = (count: number): string[] =>
  Array.from({ length: count }, (_, index) =>
    new Date(Date.UTC(2024, 2, 1 + index)).toISOString().slice(0, 10),
  );

describe('splitByProvenance', () => {
  it('одиночный imputed между двумя наблюдениями: пунктир висит на граничных наблюдениях', () => {
    const [d0, d1, d2] = dates(3);
    const split = splitByProvenance([
      point(d0, 0.3, 'observed'),
      point(d1, 0.35, 'imputed'),
      point(d2, 0.4, 'observed'),
    ]);

    // наблюдения: линия на наблюдаемых точках, разрыв на imputed
    expect(split.observedLine).toEqual([
      [d0, 0.3],
      [d1, null],
      [d2, 0.4],
    ]);
    // восстановление: включает граничные наблюдения — пунктир соединён с данными
    expect(split.imputedLine).toEqual([
      [d0, 0.3],
      [d1, 0.35],
      [d2, 0.4],
    ]);
    expect(split.imputedDots).toEqual([[d1, 0.35]]);
    expect(split.observedDots).toEqual([
      [d0, 0.3],
      [d2, 0.4],
    ]);
  });

  it('серия из 4 подряд imputed между наблюдениями: границы захвачены с обеих сторон', () => {
    const d = dates(6); // observed, 4 × imputed, observed
    const split = splitByProvenance([
      point(d[0], 0.3, 'observed'),
      point(d[1], 0.31, 'imputed'),
      point(d[2], 0.32, 'imputed'),
      point(d[3], 0.33, 'imputed'),
      point(d[4], 0.34, 'imputed'),
      point(d[5], 0.4, 'observed'),
    ]);

    expect(split.imputedLine).toEqual([
      [d[0], 0.3],
      [d[1], 0.31],
      [d[2], 0.32],
      [d[3], 0.33],
      [d[4], 0.34],
      [d[5], 0.4],
    ]);
    expect(split.imputedDots).toEqual([
      [d[1], 0.31],
      [d[2], 0.32],
      [d[3], 0.33],
      [d[4], 0.34],
    ]);
    // линия наблюдений на этом интервале разорвана
    expect(split.observedLine[2]).toEqual([d[2], null]);
  });

  it('missing даёт разрыв в ОБЕИХ сериях и не даёт точек', () => {
    const [d0, d1, d2] = dates(3);
    const split = splitByProvenance([
      point(d0, 0.3, 'observed'),
      point(d1, null, 'missing'),
      point(d2, 0.4, 'observed'),
    ]);

    expect(split.observedLine[1]).toEqual([d1, null]);
    expect(split.imputedLine[1]).toEqual([d1, null]);
    // граничные наблюдения не включаются в пунктир через пропуск
    expect(split.imputedLine).toEqual([
      [d0, null],
      [d1, null],
      [d2, null],
    ]);
    expect(split.observedDots).toEqual([
      [d0, 0.3],
      [d2, 0.4],
    ]);
    expect(split.imputedDots).toEqual([]);
  });

  it('imputed в начале ряда: без левой границы, но с правой наблюдаемой', () => {
    const [d0, d1] = dates(2);
    const split = splitByProvenance([point(d0, 0.3, 'imputed'), point(d1, 0.4, 'observed')]);
    // правое наблюдение — граница серии (isImputed слева), левое отсутствует
    expect(split.imputedLine).toEqual([
      [d0, 0.3],
      [d1, 0.4],
    ]);
  });

  it('фон отсутствует → все фоновые серии пустые разрывы', () => {
    const [d0, d1] = dates(2);
    const split = splitByProvenance([
      { ...point(d0, 0.3, 'observed'), background: null },
      { ...point(d1, 0.4, 'observed'), background: { mean: 0.35, low: 0.27, high: 0.43 } },
    ]);

    expect(split.bandLow[0]).toEqual([d0, null]);
    expect(split.bandDelta[0]).toEqual([d0, null]);
    expect(split.bgMean[0]).toEqual([d0, null]);
    // там, где фон передан: bandDelta = high − low
    expect(split.bandLow[1]).toEqual([d1, 0.27]);
    expect(split.bandDelta[1]).toEqual([d1, 0.43 - 0.27]);
    expect(split.bgMean[1]).toEqual([d1, 0.35]);
  });
});

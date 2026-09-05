import type { Limits } from '@/api/adapters/limits';
import {
  closePolygonGeometry,
  isSelfIntersecting,
  polygonAreaHa,
  toPolygonGeometry,
  validatePeriod,
  validatePolygon,
} from '@/lib/geo';
import { describe, expect, it } from 'vitest';

/**
 * Тесты lib/geo (DoD FE-3): самопересечение и минимум вершин — всегда,
 * лимиты — только когда backend их отдал (числа не хардкодятся).
 */

const square: [number, number][] = [
  [0, 0],
  [0.001, 0],
  [0.001, 0.001],
  [0, 0.001],
];

const bowtie: [number, number][] = [
  [0, 0],
  [1, 1],
  [1, 0],
  [0, 1],
];

const limits: Limits = {
  areaHaMax: 100,
  verticesMax: 10,
  periodDaysMax: 366,
  minDate: '2016-01-01',
};

describe('toPolygonGeometry', () => {
  it('замыкает кольцо', () => {
    const polygon = toPolygonGeometry(square);
    const ring = polygon.coordinates[0];
    expect(ring[0]).toEqual(ring[ring.length - 1]);
    expect(ring).toHaveLength(5);
  });

  it('уже замкнутое кольцо не дублируется', () => {
    const closed = [...square, square[0]];
    expect(toPolygonGeometry(closed).coordinates[0]).toHaveLength(5);
  });
});

describe('closePolygonGeometry', () => {
  it('closes an open outer ring before submission', () => {
    const geometry = closePolygonGeometry({
      type: 'Polygon',
      coordinates: [
        [
          [39, 45],
          [39.01, 45],
          [39.01, 45.01],
          [39, 45.01],
        ],
      ],
    });

    expect(geometry.coordinates[0]).toEqual([
      [39, 45],
      [39.01, 45],
      [39.01, 45.01],
      [39, 45.01],
      [39, 45],
    ]);
  });
});

describe('isSelfIntersecting', () => {
  it('прямоугольник без самопересечений', () => {
    expect(isSelfIntersecting(square)).toBe(false);
  });

  it('«бабочка» с самопересечением обнаруживается', () => {
    expect(isSelfIntersecting(bowtie)).toBe(true);
  });

  it('менее 4 вершин не проверяется на пересечение', () => {
    expect(isSelfIntersecting(square.slice(0, 3))).toBe(false);
  });

  it('игнорирует повторные точки свободного рисования', () => {
    expect(
      isSelfIntersecting([
        [0, 0],
        [0.001, 0],
        [0.001, 0],
        [0.001, 0.001],
        [0, 0.001],
      ]),
    ).toBe(false);
  });
});

describe('polygonAreaHa', () => {
  it('квадрат 0.001° у экватора ≈ 1.23 га', () => {
    expect(polygonAreaHa(square)).toBeGreaterThan(1.0);
    expect(polygonAreaHa(square)).toBeLessThan(1.45);
  });

  it('менее 3 вершин — площадь 0', () => {
    expect(polygonAreaHa(square.slice(0, 2))).toBe(0);
  });
});

describe('validatePolygon', () => {
  it('корректный полигон проходит без лимитов и с ними', () => {
    expect(validatePolygon(square, null).ok).toBe(true);
    expect(validatePolygon(square, limits).ok).toBe(true);
  });

  it('меньше трёх вершин — ошибка всегда, даже без лимитов', () => {
    const result = validatePolygon(square.slice(0, 2), null);
    expect(result.ok).toBe(false);
    expect(result.error).toContain('3');
  });

  it('самопересечение — ошибка даже без лимитов (corrections §1)', () => {
    expect(validatePolygon(bowtie, null).ok).toBe(false);
  });

  it('лимиты применяются только если известны', () => {
    const manyVertices: [number, number][] = Array.from({ length: 12 }, (_, i) => [i * 0.001, 0]);
    // 12 вершин: без лимитов — ok, с verticesMax 10 — ошибка
    expect(validatePolygon(manyVertices, null).ok).toBe(true);
    const limited = validatePolygon(manyVertices, limits);
    expect(limited.ok).toBe(false);
    expect(limited.error).toContain('10');
  });

  it('превышение площади ловится только с лимитом', () => {
    const big: [number, number][] = [
      [0, 0],
      [1, 0],
      [1, 1],
      [0, 1],
    ];
    expect(validatePolygon(big, null).ok).toBe(true);
    expect(validatePolygon(big, limits).ok).toBe(false);
  });
});

describe('validatePeriod', () => {
  it('from позже to — ошибка всегда', () => {
    expect(validatePeriod('2024-09-01', '2024-03-01', null).ok).toBe(false);
  });

  it('длинный период отклоняется только с лимитом', () => {
    // 399 дней — больше periodDaysMax=366 из фикстуры лимитов
    expect(validatePeriod('2024-01-01', '2025-02-04', null).ok).toBe(true);
    expect(validatePeriod('2024-01-01', '2025-02-04', limits).ok).toBe(false);
  });

  it('считает обе границы периода', () => {
    expect(validatePeriod('2024-01-01', '2024-12-31', { ...limits, periodDaysMax: 366 }).ok).toBe(
      true,
    );
    expect(validatePeriod('2024-01-01', '2024-12-31', { ...limits, periodDaysMax: 365 }).ok).toBe(
      false,
    );
  });

  it('отклоняет начало раньше минимальной даты лимитов', () => {
    expect(
      validatePeriod('2023-12-31', '2024-01-15', { ...limits, minDate: '2024-01-01' }).ok,
    ).toBe(false);
    expect(
      validatePeriod('2024-01-01', '2024-01-15', { ...limits, minDate: '2024-01-01' }).ok,
    ).toBe(true);
  });
});

import type { ReactNode } from 'react';
import { vi } from 'vitest';

if (typeof window !== 'undefined' && !window.localStorage) {
  const values = new Map<string, string>();
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
      removeItem: (key: string) => values.delete(key),
      clear: () => values.clear(),
    },
  });
}

/**
 * Общий сетап vitest.
 * jsdom не умеет WebGL и URL.createObjectURL, а импорт maplibre-gl требует
 * последний при загрузке модуля. Компонент карты подменяется заглушкой:
 * тесты каркаса проверяют роутинг и шапку, реальная карта — playwright'ом.
 */

if (typeof window !== 'undefined' && typeof window.URL.createObjectURL !== 'function') {
  Object.defineProperty(window.URL, 'createObjectURL', {
    value: () => 'blob:mock',
    configurable: true,
  });
  Object.defineProperty(window.URL, 'revokeObjectURL', {
    value: () => {},
    configurable: true,
  });
}

vi.mock('react-map-gl/maplibre', () => ({
  default: ({ children }: { children?: ReactNode }) => <div data-testid="map-stub">{children}</div>,
  Layer: () => null,
  Source: ({ children }: { children?: ReactNode }) => <>{children}</>,
}));

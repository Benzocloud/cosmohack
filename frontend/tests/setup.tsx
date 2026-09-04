import type { ReactNode } from 'react';
import { vi } from 'vitest';

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

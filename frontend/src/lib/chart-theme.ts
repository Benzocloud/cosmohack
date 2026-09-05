/**
 * Доступ к CSS-переменным дизайн-системы для canvas-библиотек
 * (ECharts, maplibre): токены §2.2 — единственный источник значений.
 * Фоллбеки — hex-значения тех же токенов для окружений без DOM (тесты).
 */

export function cssVar(name: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback;
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return value.length > 0 ? value : fallback;
}

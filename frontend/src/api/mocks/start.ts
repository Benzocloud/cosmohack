/**
 * Включение моков в браузере (corrections §3): ?mock=1 или VITE_MOCK=1.
 * В production-сборке воркер подключается только по ?mock=1 — модуль
 * импортируется динамически из main.tsx, вне мок-режима не загружается.
 */
export async function enableMockWorker(): Promise<void> {
  const { worker } = await import('./browser');
  await worker.start({ onUnhandledRequest: 'bypass', quiet: true });
}

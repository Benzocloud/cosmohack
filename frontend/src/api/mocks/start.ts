/**
 * Включение моков в браузере: ?mock=1 на dev-сервере или явная VITE_MOCK=1-сборка.
 * Обычная production-сборка не импортирует этот модуль.
 */
export async function enableMockWorker(): Promise<void> {
  const { worker } = await import('./browser');
  await worker.start({ onUnhandledRequest: 'bypass', quiet: true });
}

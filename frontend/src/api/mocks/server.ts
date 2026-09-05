import { type SetupServer, setupServer } from 'msw/node';

import { handlers } from './handlers';

/**
 * MSW для vitest (msw/node перехватывает global fetch в jsdom).
 * Модуль используется ТОЛЬКО тестами — в бандл приложения не попадает.
 */
export function setupMockServer(): SetupServer {
  return setupServer(...handlers);
}

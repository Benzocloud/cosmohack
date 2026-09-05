import { setupWorker } from 'msw/browser';

import { handlers } from './handlers';

/** Браузерный воркер MSW для dev-сервера или явной VITE_MOCK=1-сборки. */
export const worker = setupWorker(...handlers);

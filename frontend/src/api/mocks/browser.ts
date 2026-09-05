import { setupWorker } from 'msw/browser';

import { handlers } from './handlers';

/** Браузерный воркер MSW; включается только при ?mock=1 или VITE_MOCK=1. */
export const worker = setupWorker(...handlers);

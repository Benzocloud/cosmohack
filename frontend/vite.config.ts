import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
  },
  build: {
    outDir: 'dist',
    // Инвариант команды: сборка не зависит от сети к бэкенду и переменных окружения
    sourcemap: false,
  },
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.{ts,tsx}'],
    setupFiles: ['./tests/setup.tsx'],
    // В jsdom fetch требует абсолютный URL; MSW перехватывает любые хосты,
    // поэтому в тестах baseUrl задаётся через env (в билд не попадает).
    env: { VITE_API_URL: 'http://mock.local' },
  },
});

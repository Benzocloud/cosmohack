import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const frontendRoot = fileURLToPath(new URL('.', import.meta.url));

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
    proxy: {
      '/api': 'http://localhost:8080',
      '/readyz': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: {
        landing: resolve(frontendRoot, 'index.html'),
        panel: resolve(frontendRoot, 'panel.html'),
      },
    },
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

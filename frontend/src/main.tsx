import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AppRouterProvider } from './app/router';
import './styles/globals.css';

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error('root element is missing from index.html');
}

async function bootstrap() {
  // Query-параметр включает MSW только в dev; production mock требует явной сборки VITE_MOCK=1.
  // Значение параметра нормализуем от лишних кавычек (частая ошибка ручного ввода).
  const mockParam = new URLSearchParams(window.location.search)
    .get('mock')
    ?.replace(/^["']+|["']+$/g, '');
  if (import.meta.env.VITE_MOCK === '1' || (import.meta.env.DEV && mockParam === '1')) {
    const { enableMockWorker } = await import('@/api/mocks/start');
    await enableMockWorker();
  }

  createRoot(rootElement as HTMLElement).render(
    <StrictMode>
      <AppRouterProvider />
    </StrictMode>,
  );
}

void bootstrap();

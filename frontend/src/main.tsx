import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AppRouterProvider } from './app/router';
import './styles/globals.css';

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error('Не найден элемент #root в index.html');
}

createRoot(rootElement).render(
  <StrictMode>
    <AppRouterProvider />
  </StrictMode>,
);

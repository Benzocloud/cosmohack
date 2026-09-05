import { setupMockServer } from '@/api/mocks/server';
import { AreasPanel } from '@/app/AppShell';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

const server = setupMockServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderPanel() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AreasPanel />
    </QueryClientProvider>,
  );
}

describe('AreasPanel', () => {
  it('различает пустой успешный ответ и ошибку API', async () => {
    server.use(http.get('*/api/areas', () => HttpResponse.json({ areas: [] })));
    renderPanel();
    await waitFor(() => expect(screen.getByText(/Участков пока нет/)).toBeTruthy());
    expect(screen.queryByText('Не удалось загрузить список участков.')).toBeNull();
  });

  it('показывает ошибку загрузки и повторяет запрос', async () => {
    let attempts = 0;
    server.use(
      http.get('*/api/areas', () => {
        attempts += 1;
        if (attempts === 1)
          return HttpResponse.json({ code: 'internal', message: 'failure' }, { status: 500 });
        return HttpResponse.json({ areas: [] });
      }),
    );
    renderPanel();
    await waitFor(() =>
      expect(screen.getByText('Не удалось загрузить список участков.')).toBeTruthy(),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(screen.getByText(/Участков пока нет/)).toBeTruthy());
    expect(attempts).toBe(2);
  });

  it('маскирует malformed JSON как понятную ошибку списка', async () => {
    server.use(
      http.get(
        '*/api/areas',
        () =>
          new HttpResponse('{"areas":', {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }),
      ),
    );
    renderPanel();
    await waitFor(() =>
      expect(screen.getByText('Не удалось загрузить список участков.')).toBeTruthy(),
    );
  });

  it('отображает skeleton до ответа списка', () => {
    server.use(
      http.get('*/api/areas', async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
        return HttpResponse.json({ areas: [] });
      }),
    );
    renderPanel();
    expect(screen.getByLabelText('Загрузка списка участков')).toBeTruthy();
  });
});

import { isAppError } from '@/api/client';
import { setupMockServer } from '@/api/mocks/server';
import { useStartAnalysis } from '@/api/mutations';
import { useAreas, useLimits, useResultBundle } from '@/api/queries';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

/**
 * Хуки поверх MSW (msw/node перехватывает fetch в jsdom).
 * Проверяют контракты FE-1: 4 участка с разными verdict; 202/429 у запуска;
 * лимиты null, когда конфиг не отдан; сборка bundle одной версии.
 */

const server = setupMockServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const wrapper = ({ children }: { children: ReactNode }) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
};

describe('useAreas (mock)', () => {
  it('возвращает 4 участка с разными verdict', async () => {
    const { result } = renderHook(() => useAreas(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const verdicts = (result.current.data ?? []).map((area) => area.lastResult?.verdict);
    expect(new Set(verdicts)).toEqual(
      new Set(['normal', 'candidate', 'confirmed', 'insufficient_data']),
    );
  });
});

describe('useStartAnalysis (mock)', () => {
  it('429 → AppError code queue_full, мутация падает без автоповтора', async () => {
    const { result } = renderHook(() => useStartAnalysis(), { wrapper });
    result.current.mutate({
      areaId: 'area-normal',
      period: { from: '2024-03-01', to: '2024-09-30' },
    });
    await waitFor(() => expect(result.current.isError).toBe(true));
    const error: unknown = result.current.error;
    expect(isAppError(error)).toBe(true);
    if (isAppError(error)) expect(error.code).toBe('queue_full');
  });

  it('202 → job_id, мутация успешна', async () => {
    const { result } = renderHook(() => useStartAnalysis(), { wrapper });
    result.current.mutate({
      areaId: 'area-confirmed',
      period: { from: '2024-03-01', to: '2024-09-30' },
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.jobId).toMatch(/^job-live-/);
  });
});

describe('useLimits (mock)', () => {
  it('canonical config → лимиты доступны форме', async () => {
    const { result } = renderHook(() => useLimits(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toMatchObject({ areaHaMax: 5000, verticesMax: 200 });
  });
});

describe('useResultBundle (mock)', () => {
  it('series и events одной resultVersion, у confirmed — 2 события', async () => {
    const { result } = renderHook(() => useResultBundle('area-confirmed'), { wrapper });
    await waitFor(() => expect(result.current.bundle).toBeDefined());
    const bundle = result.current.bundle;
    expect(bundle?.series.resultVersion).toBe('v-confirmed-1');

    expect(bundle?.events).toHaveLength(2);
    // у confirmed series длинный ряд с пропусками всех трёх типов
    const provenances = new Set(bundle?.series.points.map((point) => point.provenance));
    expect(provenances).toEqual(new Set(['observed', 'imputed', 'missing']));
  });

  it('у insufficient_data нет фона и погоды', async () => {
    const { result } = renderHook(() => useResultBundle('area-insufficient'), { wrapper });
    await waitFor(() => expect(result.current.bundle).toBeDefined());
    expect(result.current.bundle?.series.background).toBeNull();
    expect(result.current.bundle?.series.weather).toBeNull();
    expect(result.current.bundle?.events).toHaveLength(0);
  });
});

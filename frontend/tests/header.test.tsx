import { Header } from '@/features/shell/Header';
import { SCAFFOLD } from '@/lib/labels';
import { useUi } from '@/store/ui';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

const renderHeader = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <Header />
    </QueryClientProvider>,
  );
};

afterEach(() => {
  cleanup();
  useUi.setState({ demoMode: false });
});

describe('Header', () => {
  it('показывает название приложения и пустое состояние участка', () => {
    renderHeader();
    expect(screen.getByText(SCAFFOLD.appTitle)).toBeTruthy();
    expect(screen.getByText(SCAFFOLD.noAreaSelected)).toBeTruthy();
  });

  it('показывает DemoBadge только в mock-режиме', () => {
    renderHeader();
    expect(screen.queryByTestId('demo-badge')).toBeNull();
    act(() => useUi.getState().setDemoMode(true));
    expect(screen.getByTestId('demo-badge')).toBeTruthy();
  });
});

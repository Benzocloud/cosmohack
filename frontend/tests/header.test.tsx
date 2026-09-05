import { Header } from '@/features/shell/Header';
import { SCAFFOLD } from '@/lib/labels';
import { useUi } from '@/store/ui';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

afterEach(() => {
  cleanup();
  useUi.setState({ demoMode: false });
});

describe('Header', () => {
  it('показывает название приложения и пустое состояние участка', () => {
    render(<Header />);
    expect(screen.getByText(SCAFFOLD.appTitle)).toBeTruthy();
    expect(screen.getByText(SCAFFOLD.noAreaSelected)).toBeTruthy();
  });

  it('показывает DemoBadge только в mock-режиме', () => {
    render(<Header />);
    expect(screen.queryByTestId('demo-badge')).toBeNull();
    act(() => useUi.getState().setDemoMode(true));
    expect(screen.getByTestId('demo-badge')).toBeTruthy();
  });
});

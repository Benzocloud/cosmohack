import { createAppRouter } from '@/app/router';
import { selectionActions, useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router';
import { render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

/**
 * Синхронизация выделения ↔ URL через TanStack Router (уточнение к FE-0):
 * router — единственный источник истины по search; zustand — только зеркало.
 */

afterEach(() => {
  // Роутеры разных тестов не должны делить регистр selectionApi и состояние stores
  useSelection.setState({ selectedAreaId: null, selectedEventId: null, selectedDate: null });
  useUi.setState({ demoMode: false, mobileTab: 'areas', cardOpen: false, listCollapsed: false });
});

describe('синхронизация выделения с URL', () => {
  it('восстанавливает выделение и mock-режим из search при загрузке', async () => {
    const history = createMemoryHistory({
      initialEntries: ['/?area=a1&event=e1&date=2024-05-01&mock=1'],
    });
    const router = createAppRouter(history);
    render(<RouterProvider router={router} />);

    await new Promise((resolve) => setTimeout(resolve, 0));
    const selection = useSelection.getState();
    expect(selection.selectedAreaId).toBe('a1');
    expect(selection.selectedEventId).toBe('e1');
    expect(selection.selectedDate).toBe('2024-05-01');
    expect(useUi.getState().demoMode).toBe(true);
  });

  it('selectArea пишет area в URL, сохраняя mock', async () => {
    const history = createMemoryHistory({ initialEntries: ['/?mock=1'] });
    const router = createAppRouter(history);
    render(<RouterProvider router={router} />);

    await new Promise((resolve) => setTimeout(resolve, 0));
    selectionActions.selectArea('a2');

    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(router.state.location.search.area).toBe('a2');
    expect(router.state.location.search.mock).toBe('1');
    expect(useSelection.getState().selectedAreaId).toBe('a2');
  });

  it('сохраняет маршрут панели и search при изменении выделения', async () => {
    const history = createMemoryHistory({
      initialEntries: ['/panel.html?mock=1&dev=states'],
    });
    const router = createAppRouter(history);
    render(<RouterProvider router={router} />);

    await new Promise((resolve) => setTimeout(resolve, 0));
    selectionActions.selectArea('a2');
    await new Promise((resolve) => setTimeout(resolve, 0));
    selectionActions.selectEvent('e2');
    await new Promise((resolve) => setTimeout(resolve, 0));
    selectionActions.selectDate('2024-06-01');

    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(router.state.location.pathname).toBe('/panel.html');
    expect(router.state.location.search).toMatchObject({
      area: 'a2',
      event: 'e2',
      date: '2024-06-01',
      mock: '1',
      dev: 'states',
    });
  });

  it('null снимает параметр с URL', async () => {
    const history = createMemoryHistory({ initialEntries: ['/?area=a1&event=e1'] });
    const router = createAppRouter(history);
    render(<RouterProvider router={router} />);

    await new Promise((resolve) => setTimeout(resolve, 0));
    selectionActions.selectEvent(null);

    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(router.state.location.search.event).toBeUndefined();
    expect(router.state.location.search.area).toBe('a1');
    expect(useSelection.getState().selectedEventId).toBeNull();
  });
});

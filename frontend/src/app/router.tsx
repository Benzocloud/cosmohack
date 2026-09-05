import { type SelectionPatch, registerSelectionApi, useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';
import {
  Outlet,
  type RouterHistory,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  useRouter,
  useSearch,
} from '@tanstack/react-router';
import { type ComponentType, type LazyExoticComponent, Suspense, lazy, useEffect } from 'react';
import { AppShell } from './AppShell';
import { Providers } from './providers';

// Страница состояний — только для dev (?dev=states, corrections §2).
// Ветка с dynamic import вырезается при production-сборке:
// import.meta.env.DEV заменяется на false и chunk не попадает в dist.
let StatesPageLazy: LazyExoticComponent<ComponentType> | null = null;
if (import.meta.env.DEV) {
  StatesPageLazy = lazy(() => import('@/features/dev/StatesPage'));
}

/**
 * Панель доступна на /panel.html; состояние живёт в query-параметрах — area,
 * event, date, mock, dev. Корень приложения занят самостоятельным лендингом.
 */
export interface AppSearch {
  area?: string;
  event?: string;
  date?: string;
  mock?: string;
  dev?: string;
}

// TanStack парсит значения search как JSON: «?mock=1» приходит числом 1,
// поэтому числа нормализуем обратно в строки (иначе параметр теряется).
const str = (value: unknown): string | undefined => {
  if (typeof value === 'string' && value.length > 0) return value;
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return undefined;
};

/**
 * Мост роутер ↔ stores. Поисковые параметры — единственный источник истины:
 * сюда значения приходят из useSearch, а записи идут через selectionActions → router.navigate.
 */
function SearchSync() {
  const search = useSearch({ strict: false }) as AppSearch;
  const router = useRouter();

  useEffect(() => {
    useSelection.getState().setFromSearch(search);
    useUi.getState().setDemoMode(search.mock === '1' || import.meta.env.VITE_MOCK === '1');
  }, [search]);

  useEffect(() => {
    registerSelectionApi({
      navigate: (patch: SelectionPatch) => {
        // Функциональная форма search у router.navigate сводит тип к never,
        // поэтому следующий search собираем из состояния роутера на момент вызова —
        // не из снапшота рендера, иначе теряем параметры (mock/dev). null снимает ключ.
        const current = router.state.location.search as AppSearch;
        const next: AppSearch = { ...current };
        for (const [key, value] of Object.entries(patch)) {
          if (value == null) {
            delete next[key as keyof AppSearch];
          } else {
            next[key as keyof AppSearch] = value as string;
          }
        }
        const currentPath = router.state.location.pathname === '/panel.html' ? '/panel.html' : '/';
        router.navigate({ to: currentPath, search: next, replace: true });
      },
    });
  }, [router]);

  return null;
}

const rootRoute = createRootRoute({
  validateSearch: (search: Record<string, unknown>): AppSearch => ({
    area: str(search.area),
    event: str(search.event),
    date: str(search.date),
    mock: str(search.mock),
    dev: str(search.dev),
  }),
  component: () => (
    <>
      <SearchSync />
      <Outlet />
    </>
  ),
});

function IndexPage() {
  const search = useSearch({ strict: false }) as AppSearch;
  if (import.meta.env.DEV && search.dev === 'states' && StatesPageLazy) {
    return (
      <Providers>
        <Suspense fallback={null}>
          <StatesPageLazy />
        </Suspense>
      </Providers>
    );
  }
  return (
    <Providers>
      <AppShell />
    </Providers>
  );
}

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: IndexPage,
});

const panelRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/panel.html',
  component: IndexPage,
});

const routeTree = rootRoute.addChildren([indexRoute, panelRoute]);

/** Фабрика для тестов: memory history изолирует URL между тестами. */
export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history });
}

export const router = createAppRouter();

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

export function AppRouterProvider() {
  return <RouterProvider router={router} />;
}

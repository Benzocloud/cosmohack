import { create } from 'zustand';

/** Состояние компоновки UI (frontend-plan §5 store/ui). */
export type MobileTab = 'areas' | 'map' | 'analysis';
export type Basemap = 'map' | 'satellite';

const BASEMAP_STORAGE_KEY = 'agropulse.basemap';

/**
 * demoMode известен синхронно до первого рендера: карта в моке должна
 * стартовать над фикстурными контурами, а эффект SearchSync опаздывает.
 */
function initialDemoMode(): boolean {
  if (typeof window === 'undefined') return false;
  if (import.meta.env.VITE_MOCK === '1') return true;
  if (!import.meta.env.DEV) return false;
  const mock = new URLSearchParams(window.location.search)
    .get('mock')
    ?.replace(/^["']+|["']+$/g, '');
  return mock === '1';
}

function readStoredBasemap(): Basemap {
  if (typeof window === 'undefined') return 'map';
  return window.localStorage.getItem(BASEMAP_STORAGE_KEY) === 'satellite' ? 'satellite' : 'map';
}

export interface UiState {
  mobileTab: MobileTab;
  /** Планшет: список свёрнут в рейку, полный список открывается Sheet'ом. */
  listCollapsed: boolean;
  /** Планшет/мобиль: карточка участка открыта поверх карты. */
  cardOpen: boolean;
  /** Dev ?mock=1 или явная VITE_MOCK=1-сборка: фикстуры + бейдж. */
  demoMode: boolean;
  /** Подложка карты; выбор сохраняется в localStorage (FE-3). */
  basemap: Basemap;
  /** Участок, у которого сохранение прошло, а запуск не удался: в списке кнопка «Запустить анализ». */
  pendingStart: { areaId: string; period: { from: string; to: string } } | null;
  /** Incremented when a user requests drawing from outside the map tab. */
  drawRequest: number;
  setMobileTab: (tab: MobileTab) => void;
  setListCollapsed: (collapsed: boolean) => void;
  setCardOpen: (open: boolean) => void;
  setDemoMode: (demo: boolean) => void;
  setBasemap: (basemap: Basemap) => void;
  setPendingStart: (
    pendingStart: { areaId: string; period: { from: string; to: string } } | null,
  ) => void;
  requestDraw: () => void;
}

export const useUi = create<UiState>()((set) => ({
  mobileTab: 'areas',
  listCollapsed: false,
  cardOpen: false,
  demoMode: initialDemoMode(),
  basemap: readStoredBasemap(),
  pendingStart: null,
  drawRequest: 0,
  setMobileTab: (mobileTab) => set({ mobileTab }),
  setListCollapsed: (listCollapsed) => set({ listCollapsed }),
  setCardOpen: (cardOpen) => set({ cardOpen }),
  setDemoMode: (demoMode) => set({ demoMode }),
  setBasemap: (basemap) => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(BASEMAP_STORAGE_KEY, basemap);
    }
    set({ basemap });
  },
  setPendingStart: (pendingStart) => set({ pendingStart }),
  requestDraw: () => set((state) => ({ drawRequest: state.drawRequest + 1 })),
}));

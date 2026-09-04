import { create } from 'zustand';

/** Состояние компоновки UI (frontend-plan §5 store/ui). */
export type MobileTab = 'areas' | 'map' | 'analysis';

export interface UiState {
  mobileTab: MobileTab;
  /** Планшет: список свёрнут в рейку, полный список открывается Sheet'ом. */
  listCollapsed: boolean;
  /** Планшет/мобиль: карточка участка открыта поверх карты. */
  cardOpen: boolean;
  /** ?mock=1 или VITE_MOCK=1: фикстуры + бейдж «Демонстрационные данные». */
  demoMode: boolean;
  setMobileTab: (tab: MobileTab) => void;
  setListCollapsed: (collapsed: boolean) => void;
  setCardOpen: (open: boolean) => void;
  setDemoMode: (demo: boolean) => void;
}

export const useUi = create<UiState>()((set) => ({
  mobileTab: 'areas',
  listCollapsed: false,
  cardOpen: false,
  demoMode: false,
  setMobileTab: (mobileTab) => set({ mobileTab }),
  setListCollapsed: (listCollapsed) => set({ listCollapsed }),
  setCardOpen: (cardOpen) => set({ cardOpen }),
  setDemoMode: (demoMode) => set({ demoMode }),
}));

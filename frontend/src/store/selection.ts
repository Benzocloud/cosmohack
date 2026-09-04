import { create } from 'zustand';

/**
 * Зеркало выделения (frontend-plan §9).
 * Единственный источник истины по search — TanStack Router: значения сюда
 * доставляет SearchSync из маршрута, а записи идут через selectionActions → router.navigate.
 * Прямые записи history.replaceState запрещены (уточнение к FE-0).
 */

export interface SelectionState {
  selectedAreaId: string | null;
  selectedEventId: string | null;
  selectedDate: string | null;
  setFromSearch: (s: { area?: string; event?: string; date?: string }) => void;
}

export const useSelection = create<SelectionState>()((set) => ({
  selectedAreaId: null,
  selectedEventId: null,
  selectedDate: null,
  setFromSearch: ({ area, event, date }) =>
    set({
      selectedAreaId: area ?? null,
      selectedEventId: event ?? null,
      selectedDate: date ?? null,
    }),
}));

export type SelectionPatch = {
  area?: string | null;
  event?: string | null;
  date?: string | null;
};

interface SelectionApi {
  navigate: (patch: SelectionPatch) => void;
}

let selectionApi: SelectionApi | null = null;

/** Вызывается один раз из SearchSync: подключает запись выделения к истории роутера. */
export function registerSelectionApi(api: SelectionApi): void {
  selectionApi = api;
}

/** Действия для компонентов: патчат search через роутер; null снимает параметр. */
export const selectionActions = {
  selectArea: (areaId: string | null) => selectionApi?.navigate({ area: areaId }),
  selectEvent: (eventId: string | null) => selectionApi?.navigate({ event: eventId }),
  selectDate: (date: string | null) => selectionApi?.navigate({ date }),
};

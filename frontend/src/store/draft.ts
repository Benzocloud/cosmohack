import { create } from 'zustand';

/**
 * Режим рисования полигона и черновик геометрии (frontend-plan §5 store/draft, §8).
 * Вершины синхронизируются из terra-draw ('change' → setVertices); источник
 * готовой геометрии — событие 'finish' (setFinishedGeometry) → AddAreaDialog.
 * Валидация выполняется в lib/geo.ts с лимитами из useLimits (corrections §1).
 */
export type DrawMode = 'off' | 'drawing';

export interface DraftState {
  drawMode: DrawMode;
  /** Черновик: вершины [longitude, latitude] в порядке ввода (без замыкания). */
  vertices: [number, number][];
  validationError: string | null;
  /** Завершённый черновик: открытый AddAreaDialog предзаполняется им. */
  finishedGeometry: GeoJSON.Polygon | null;
  startDrawing: () => void;
  cancelDrawing: () => void;
  setVertices: (vertices: [number, number][]) => void;
  setValidationError: (error: string | null) => void;
  setFinishedGeometry: (geometry: GeoJSON.Polygon | null) => void;
}

export const useDraft = create<DraftState>()((set) => ({
  drawMode: 'off',
  vertices: [],
  validationError: null,
  finishedGeometry: null,
  startDrawing: () =>
    set({ drawMode: 'drawing', vertices: [], validationError: null, finishedGeometry: null }),
  cancelDrawing: () =>
    set({ drawMode: 'off', vertices: [], validationError: null, finishedGeometry: null }),
  setVertices: (vertices) => set({ vertices }),
  setValidationError: (validationError) => set({ validationError }),
  setFinishedGeometry: (finishedGeometry) => set({ finishedGeometry }),
}));

import { create } from 'zustand';

/**
 * Режим рисования полигона и черновик геометрии (frontend-plan §5 store/draft).
 * Каркас FE-0: состояние и операции без привязки к карте (карта — FE-3).
 * Валидация (самопересечение, вершины, площадь) появится в lib/geo.ts на FE-3.
 */
export type DrawMode = 'off' | 'drawing';

export interface DraftState {
  drawMode: DrawMode;
  /** Черновик: вершины [longitude, latitude] в порядке ввода. */
  vertices: [number, number][];
  validationError: string | null;
  startDrawing: () => void;
  cancelDrawing: () => void;
  addVertex: (lng: number, lat: number) => void;
  undoVertex: () => void;
  setValidationError: (error: string | null) => void;
}

export const useDraft = create<DraftState>()((set) => ({
  drawMode: 'off',
  vertices: [],
  validationError: null,
  startDrawing: () => set({ drawMode: 'drawing', vertices: [], validationError: null }),
  cancelDrawing: () => set({ drawMode: 'off', vertices: [], validationError: null }),
  addVertex: (lng, lat) =>
    set((state) =>
      state.drawMode === 'drawing'
        ? { vertices: [...state.vertices, [lng, lat] as [number, number]] }
        : state,
    ),
  undoVertex: () => set((state) => ({ vertices: state.vertices.slice(0, -1) })),
  setValidationError: (validationError) => set({ validationError }),
}));

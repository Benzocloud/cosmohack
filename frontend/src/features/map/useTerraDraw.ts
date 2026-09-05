import type { Map as MaplibreMap } from 'maplibre-gl';
import { useEffect, useRef } from 'react';
import { TerraDraw, TerraDrawFreehandMode, TerraDrawSelectMode } from 'terra-draw';
import { TerraDrawMapLibreGLAdapter } from 'terra-draw-maplibre-gl-adapter';

/**
 * Жизненный цикл terra-draw: один экземпляр на карту, режим freehand — только
 * пока drawMode==='drawing'. Отпускание pointer/touch автоматически замыкает контур.
 *
 * Особенности версии 1.33 (проверено по dist .d.ts):
 *  - публичного finish() нет — полигон закрывается клавиатурным событием Enter
 *    (режим слушает keyup), поэтому «Завершить» диспатчит его на document;
 *  - «Отменить точку» — публичный session-undo (draw.undo()), он снимает
 *    последнюю поставленную вершину;
 *  - черновик в снапшоте помечен свойством currentlyDrawing; после поставленных
 *    вершин идут координата курсора и превью-замыкание (см. draftVertices).
 */

export type Ring = [number, number][];

interface UseTerraDrawOptions {
  map: MaplibreMap | null;
  /** Подложка: смена style удаляет слои адаптера, экземпляр нужно пересоздать. */
  basemap: string;
  drawing: boolean;
  /** Синхронизация поставленных вершин черновика в draft-store. */
  onVerticesChange?: (vertices: Ring) => void;
  /** Полигон закрыт (кнопка «Завершить»/двойной клик/клик по первой точке). */
  onFinish?: (ring: Ring) => void;
  onIncomplete?: () => void;
}

function draftFeature(draw: TerraDraw) {
  return draw
    .getSnapshot()
    .find(
      (feature) =>
        feature.properties?.currentlyDrawing === true &&
        (feature.properties?.mode === 'freehand' || feature.properties?.mode === 'polygon'),
    );
}

/** Черновик: поставленные вершины. Формат черновика terra-draw (проверено на 1.33):
 *  [...вершины, координата курсора, превью-замыкание к первой вершине] —
 *  поэтому поставленных = coordinates.length − 2. */
function draftVertices(draw: TerraDraw): Ring {
  const feature = draftFeature(draw);
  if (!feature || feature.geometry.type !== 'Polygon') return [];
  const coordinates = feature.geometry.coordinates[0] as Ring;
  return coordinates.length > 2 ? coordinates.slice(0, -2) : [];
}

/**
 * Возвращает вершины в черновик: серия pointer/click-событий по canvas
 * с координатами, спроецированными из lng/lat.
 */
function replayClicks(map: MaplibreMap, ring: Ring): void {
  const canvas = map.getCanvas();
  const rect = canvas.getBoundingClientRect();
  let delay = 40;
  for (const [lng, lat] of ring) {
    const point = map.project([lng, lat]);
    const base = {
      clientX: rect.left + point.x,
      clientY: rect.top + point.y,
      bubbles: true,
      cancelable: true,
      button: 0,
      pointerId: 1,
      isPrimary: true,
      pointerType: 'mouse',
    };
    window.setTimeout(() => {
      canvas.dispatchEvent(new PointerEvent('pointerdown', { ...base, buttons: 1 }));
      canvas.dispatchEvent(new MouseEvent('mousedown', { ...base, buttons: 1 }));
      canvas.dispatchEvent(new PointerEvent('pointerup', { ...base, buttons: 0 }));
      canvas.dispatchEvent(new MouseEvent('mouseup', { ...base, buttons: 0 }));
      canvas.dispatchEvent(new MouseEvent('click', base));
    }, delay);
    delay += 70;
  }
}

export function useTerraDraw({
  map,
  basemap,
  drawing,
  onVerticesChange,
  onFinish,
  onIncomplete,
}: UseTerraDrawOptions) {
  const drawRef = useRef<TerraDraw | null>(null);
  const mapLatest = useRef<MaplibreMap | null>(map);
  mapLatest.current = map;
  const initializedMap = useRef<MaplibreMap | null>(null);
  const initializedBasemap = useRef<string | null>(null);
  const initializedStyle = useRef<ReturnType<MaplibreMap['getStyle']> | null>(null);
  const drawingLatest = useRef(drawing);
  drawingLatest.current = drawing;
  const finishRef = useRef(onFinish);
  finishRef.current = onFinish;
  const verticesRef = useRef(onVerticesChange);
  verticesRef.current = onVerticesChange;
  const incompleteRef = useRef(onIncomplete);
  incompleteRef.current = onIncomplete;

  // Один экземпляр на карту и подложку. MapLibre удаляет пользовательские
  // source/layer при setStyle, поэтому ждём новый style и регистрируем адаптер заново.
  useEffect(() => {
    if (!map) return undefined;

    let cancelled = false;
    let onPointerUp: (() => void) | undefined;
    let draw: TerraDraw | undefined;

    const initialize = () => {
      if (cancelled) return;
      draw = new TerraDraw({
        adapter: new TerraDrawMapLibreGLAdapter({ map, prefixId: `td-${basemap}` }),
        // StaticMode в v1.33 нет: «спокойный» режим — SelectMode
        modes: [
          new TerraDrawFreehandMode({
            minDistance: 4,
            smoothing: 0.35,
            autoClose: false,
            drawInteraction: 'click-drag',
            styles: {
              closingPointOpacity: 0,
              closingPointOutlineOpacity: 0,
              closingPointWidth: 0,
              closingPointOutlineWidth: 0,
            },
          }),
          new TerraDrawSelectMode(),
        ],
      });
      draw.start();
      draw.setMode(drawingLatest.current ? 'freehand' : 'select');
      drawRef.current = draw;
      initializedMap.current = map;
      initializedBasemap.current = basemap;
      initializedStyle.current = map.getStyle();
      if (import.meta.env.DEV) {
        (window as unknown as { __agroDraw?: TerraDraw }).__agroDraw = draw;
      }

      draw.on('change', () => {
        if (draw?.getMode() !== 'freehand') return;
        verticesRef.current?.(draftVertices(draw));
      });
      draw.on('finish', (id, context) => {
        if (context.action !== 'draw' || !draw) return;
        const feature = draw.getSnapshotFeature(id);
        if (!feature || feature.geometry.type !== 'Polygon') return;
        const closed = feature.geometry.coordinates[0] as Ring;
        // закрытое кольцо: последняя точка дублирует первую — отдаем незамкнутый набор
        const ring = closed.slice(0, -1);
        verticesRef.current?.([]);
        finishRef.current?.(ring);
      });

      // При отпускании свободной линии без возврата к началу Terra Draw может
      // оставить черновик открытым. Ждём завершения нативного dragend и не
      // показываем ошибку для случайного касания с одной-двумя вершинами.
      onPointerUp = () => {
        queueMicrotask(() => {
          if (draw?.getMode() !== 'freehand') return;
          const draft = draftFeature(draw);
          if (!draft) return;
          if (draftVertices(draw).length >= 3) {
            incompleteRef.current?.();
            return;
          }
          // Короткое касание не образует полигона. Убираем такой черновик,
          // чтобы следующий жест мог начать рисование заново.
          draw.clear();
          verticesRef.current?.([]);
        });
      };
      map.getCanvas().addEventListener('pointerup', onPointerUp);
    };

    const previousStyle = initializedStyle.current;
    let retryTimer: number | undefined;
    const waitForStyle = () => {
      if (cancelled || draw) return;
      const styleChanged =
        initializedMap.current !== map ||
        initializedBasemap.current === null ||
        map.getStyle() !== previousStyle;
      if (map.isStyleLoaded() && styleChanged) {
        map.off('style.load', waitForStyle);
        if (retryTimer !== undefined) window.clearTimeout(retryTimer);
        initialize();
        return;
      }
      retryTimer = window.setTimeout(waitForStyle, 50);
    };
    // При setStyle старый style иногда остаётся «loaded» в момент эффекта,
    // а событие style.load уже могло прийти до подписки. Проверяем одновременно
    // событие и смену объекта style, чтобы адаптер не остался без слоёв TerraDraw.
    map.once('style.load', waitForStyle);
    waitForStyle();

    return () => {
      cancelled = true;
      map.off('style.load', waitForStyle);
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
      if (onPointerUp) map.getCanvas().removeEventListener('pointerup', onPointerUp);
      try {
        draw?.stop();
      } catch {
        // setStyle уже мог удалить terra-draw layers до React cleanup.
      }
      if (drawRef.current === draw) drawRef.current = null;
    };
  }, [map, basemap]);

  // Пересоздание чертёжных слоёв после смены подложки: setStyle уничтожает
  // слои адаптера, поэтому экземпляр пересоздаётся на каждом basemap-ключе
  // (см. MapView: ключ передаётся через dependency mapInstance).
  // map в зависимостях обязателен: пользователь может включить рисование до
  // загрузки карты — тогда режим выставляется, как только экземпляр появился
  useEffect(() => {
    const draw = drawRef.current;
    if (!draw || !map) return;
    if (drawing) {
      draw.clear();
      draw.setMode('freehand');
    } else {
      draw.setMode('select');
    }
  }, [drawing, map]);

  return {
    /**
     * Программное завершение. Основной путь — keyup Enter элементам карты
     * (режим закрывает полигон сам); подстраховка: если черновик всё ещё
     * в снапшоте, убираем его и завершаем вручную (план §8, фолбэк).
     */
    finishDrawing(): void {
      const currentMap = mapLatest.current;
      const draw = drawRef.current;
      if (!draw || !currentMap || draw.getMode() !== 'polygon') return;
      const ring = draftVertices(draw);
      if (ring.length < 3) return;
      const draft = draftFeature(draw);
      for (const element of [currentMap.getCanvasContainer(), currentMap.getCanvas()]) {
        element.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', bubbles: true }));
      }
      window.setTimeout(() => {
        if (!drawRef.current) return;
        const stillDrawing = draftFeature(drawRef.current);
        if (!stillDrawing || String(stillDrawing.id) !== String(draft?.id)) return; // режим закрыл сам
        // id определён: выше сравнили с draft?.id, но тип снапшота допускает undefined
        drawRef.current.removeFeatures([stillDrawing.id as never]);
        verticesRef.current?.([]);
        finishRef.current?.(ring);
      }, 120);
    },
    /**
     * «Отменить точку». Session-undo в v1.33 не откатывает вершины в режиме
     * рисования, а прямая правка геометрии ломает внутренний счётчик режима.
     * Поэтому черновик пересоздаётся, а оставшиеся вершины возвращаются
     * синтетическими кликами по canvas (maplibre не требует trusted events).
     */
    undoVertex(): void {
      const draw = drawRef.current;
      const currentMap = mapLatest.current;
      if (!draw || !currentMap || draw.getMode() !== 'freehand') return;
      const remaining = draftVertices(draw).slice(0, -1);
      draw.clear();
      draw.setMode('freehand');
      verticesRef.current?.([]);
      if (remaining.length > 0) {
        replayClicks(currentMap, remaining);
      }
    },
    /** Полная очистка чертёжных фич (после сохранения/отмены диалога). */
    clear(): void {
      drawRef.current?.clear();
    },
  };
}

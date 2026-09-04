import type { StyleSpecification } from 'maplibre-gl';

/**
 * Подложки карты (уточнение FE-3): Positron — «Карта», Esri World Imagery — «Спутник».
 * Атрибуция OSM/CARTO и контуров OSM добавляется в MapView как customAttribution —
 * она должна быть видна на обеих подложках.
 */

export const POSITRON_STYLE_URL = 'https://basemaps.cartocdn.com/gl/positron-gl-style/style.json';

/** Запасная подложка (frontend-plan §1) — используется при ошибке загрузки Positron. */
export const OPENFREEMAP_STYLE_URL = 'https://tiles.openfreemap.org/styles/positron';

export const SATELLITE_STYLE: StyleSpecification = {
  version: 8,
  sources: {
    esri: {
      type: 'raster',
      tiles: [
        'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
      ],
      tileSize: 256,
      attribution: 'Esri, Maxar, Earthstar Geographics, and the GIS User Community',
    },
  },
  layers: [{ id: 'esri-imagery', type: 'raster', source: 'esri' }],
};

/** Начальный вид: центр РФ (frontend-plan §8). */
export const DEFAULT_MAP_VIEW = { longitude: 60, latitude: 57, zoom: 4 };

/** В mock-режиме стартуем над фикстурными контурами (Краснодарский край). */
export const MOCK_MAP_VIEW = { longitude: 39.0, latitude: 45.65, zoom: 11.3 };

const VIEW_STORAGE_KEY = 'agropulse.map-view';

export interface PersistedMapView {
  longitude: number;
  latitude: number;
  zoom: number;
}

export function readLastMapView(): PersistedMapView | null {
  if (typeof window === 'undefined') return null;
  try {
    const raw = window.localStorage.getItem(VIEW_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as PersistedMapView;
    if (
      typeof parsed.longitude !== 'number' ||
      typeof parsed.latitude !== 'number' ||
      typeof parsed.zoom !== 'number'
    ) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function saveLastMapView(view: PersistedMapView): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(VIEW_STORAGE_KEY, JSON.stringify(view));
  } catch {
    // переполнение/недоступность localStorage не ломают карту
  }
}

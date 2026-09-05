import ReactECharts from 'echarts-for-react';

import type { Series } from '@/api/types';
import { cssVar } from '@/lib/chart-theme';
import { splitByProvenance } from '@/lib/series';

/**
 * Превью-график NDVI для dev-страницы состояний (FE-1) — проверка данных
 * через lib/series.ts. НЕ финальный NdviChart (он на FE-4): без легенды,
 * тултипов, zoom и событий. Цвета — из токенов §2.2 с hex-фоллбеком для SSR/тестов.
 */

export function PreviewChart({ series }: { series: Series }) {
  const split = splitByProvenance(series.points);
  const observed = cssVar('--observed', '#0E7C6B');
  const imputed = cssVar('--imputed', '#7A3FE0');
  const band = cssVar('--background-band', 'rgba(100,116,139,.16)');
  const bgMean = cssVar('--background-mean', '#64748B');
  const gridLine = cssVar('--bg-muted', '#EEF1F5');
  const axisText = cssVar('--fg-tertiary', '#7B8794');

  const option = {
    animation: false,
    xAxis: {
      type: 'time',
      axisLabel: { hideOverlap: true, color: axisText },
      axisLine: { lineStyle: { color: gridLine } },
    },
    yAxis: {
      type: 'value',
      name: 'NDVI',
      nameTextStyle: { color: axisText },
      // отрицательные значения не обрезаем (бриф §4)
      min: (extent: { min?: number }) => Math.min(-0.1, Math.floor((extent.min ?? 0) * 10) / 10),
      max: (extent: { max?: number }) => Math.max(1, Math.ceil((extent.max ?? 1) * 10) / 10),
      splitLine: { lineStyle: { color: gridLine } },
      axisLabel: { color: axisText },
    },
    grid: { left: 48, right: 16, top: 24, bottom: 28 },
    series: [
      // сезонный фон — полоса p10–p90 (стек из двух линий)
      {
        id: 'bandLow',
        type: 'line',
        stack: 'band',
        data: split.bandLow,
        lineStyle: { opacity: 0 },
        symbol: 'none',
        silent: true,
      },
      {
        id: 'bandHigh',
        type: 'line',
        stack: 'band',
        data: split.bandDelta,
        lineStyle: { opacity: 0 },
        symbol: 'none',
        areaStyle: { color: band },
        silent: true,
      },
      {
        id: 'bgMean',
        type: 'line',
        data: split.bgMean,
        lineStyle: { type: 'dashed', width: 1.5, color: bgMean },
        symbol: 'none',
      },
      {
        id: 'imputedLine',
        type: 'line',
        data: split.imputedLine,
        connectNulls: false,
        lineStyle: { type: [4, 4], width: 1.75, color: imputed },
        symbol: 'none',
        z: 3,
      },
      {
        id: 'observedLine',
        type: 'line',
        data: split.observedLine,
        connectNulls: false,
        lineStyle: { width: 2, color: observed },
        symbol: 'none',
        z: 4,
      },
      {
        id: 'imputedDots',
        type: 'scatter',
        data: split.imputedDots,
        symbol: 'diamond',
        symbolSize: 8,
        itemStyle: { color: '#FFFFFF', borderColor: imputed, borderWidth: 1.75 },
        z: 5,
      },
      {
        id: 'observedDots',
        type: 'scatter',
        data: split.observedDots,
        symbol: 'circle',
        symbolSize: 6,
        itemStyle: { color: observed },
        z: 6,
      },
    ],
  };

  return <ReactECharts option={option} notMerge style={{ height: 280 }} lazyUpdate />;
}

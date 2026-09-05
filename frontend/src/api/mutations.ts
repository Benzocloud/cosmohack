import { useMutation, useQueryClient } from '@tanstack/react-query';

import { type AreaRaw, adaptArea } from '@/api/adapters/areas';
import { apiDelete, apiPost } from '@/api/client';
import type { Area, Period } from '@/api/types';
import { closePolygonGeometry } from '@/lib/geo';

/**
 * Мутации (frontend-plan §4, corrections §1).
 * Автоповторы запрещены (retry: false); кнопки блокируются isPending в UI.
 */

export interface CreateAreaInput {
  name: string;
  geometry: GeoJSON.Polygon;
  sourceKind: 'contour' | 'drawn';
  sourceLabel: string;
  sourceExternalId?: string;
  sourceProvider?: string;
  cropType?: string;
  period: Period;
}

/** Запуск анализа: успех = 202 + {job_id}; 429 → AppError code 'queue_full' без автоповтора. */
async function startAnalysis(input: { areaId: string; period: Period }): Promise<{
  jobId: string;
}> {
  const body = await apiPost<{ job_id?: string }>(
    '/api/areas/{id}/analyses' as never,
    { period: input.period },
    {
      id: input.areaId,
    },
  );
  if (!body.job_id) {
    throw new Error('startAnalysis: 202 response is missing job_id');
  }
  return { jobId: body.job_id };
}

export function useStartAnalysis() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: startAnalysis,
    retry: false,
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['area', variables.areaId] });
      void queryClient.invalidateQueries({ queryKey: ['areas'] });
    },
  });
}

async function createArea(input: CreateAreaInput): Promise<Area> {
  const raw = await apiPost<AreaRaw>('/api/areas' as never, {
    name: input.name,
    geometry: closePolygonGeometry(input.geometry),
    source: {
      kind: input.sourceKind,
      label: input.sourceLabel,
      ...(input.sourceKind === 'contour' ? { contour_id: input.sourceExternalId } : {}),
      ...(input.sourceKind === 'contour' && input.sourceProvider
        ? { provider: input.sourceProvider }
        : {}),
      ...(input.cropType?.trim() ? { crop_type: input.cropType.trim() } : {}),
    },
    period: input.period,
  });
  return adaptArea(raw);
}

export function useCreateArea() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createArea,
    retry: false,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['areas'] });
    },
  });
}

async function deleteArea(areaId: string): Promise<void> {
  await apiDelete('/api/areas/{id}' as never, { id: areaId });
}

export function useDeleteArea() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteArea,
    retry: false,
    onSuccess: (_data, areaId) => {
      // Удалённый участок и все его результаты больше не читаем
      queryClient.removeQueries({ queryKey: ['area', areaId] });
      void queryClient.invalidateQueries({ queryKey: ['areas'] });
    },
  });
}

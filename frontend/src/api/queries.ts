import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';

import { type AreaRaw, adaptArea, adaptAreaList } from '@/api/adapters/areas';
import { type ContoursRaw, type ContoursResult, adaptContours } from '@/api/adapters/contours';
import { type EventRaw, adaptEventList } from '@/api/adapters/events';
import { type JobRaw, adaptJob } from '@/api/adapters/jobs';
import {
  type Limits,
  type LimitsRaw,
  adaptLimits,
  adaptLimitsFromErrorBody,
} from '@/api/adapters/limits';
import { type SeriesRaw, adaptSeries } from '@/api/adapters/series';
import { apiGet, isAppError } from '@/api/client';
import type { AnalysisEvent, Area, JobMeta, Series } from '@/api/types';
import { useEffect } from 'react';

/**
 * TanStack Query-хуки (frontend-plan §9, corrections §1).
 * Компоненты получают только view-модели; сырой JSON остаётся внутри queryFn.
 */

/** Опрос задачи, пока она queued/running (corrections §1: только polling, 2 с). */
export const JOB_POLL_MS = 2000;
/** Верхняя граница backoff при потере связи (corrections §1: до 10 с). */
export const JOB_BACKOFF_MAX_MS = 10000;

async function fetchAreas(): Promise<Area[]> {
  const raw = await apiGet<unknown>('/api/areas');
  return adaptAreaList(raw);
}

async function fetchArea(areaId: string): Promise<Area> {
  const raw = await apiGet<AreaRaw>('/api/areas/{id}', { path: { id: areaId } });
  return adaptArea(raw);
}

async function fetchContours(bbox: string, mockCase?: string): Promise<ContoursResult> {
  const raw = await apiGet<ContoursRaw>('/api/regions/contours', {
    query: { bbox, mock_case: mockCase },
  });
  return adaptContours(raw);
}

async function fetchJob(jobId: string): Promise<JobMeta> {
  const raw = await apiGet<JobRaw>('/api/jobs/{id}', { path: { id: jobId } });
  return adaptJob(raw);
}

async function fetchSeries(areaId: string, version?: string): Promise<Series> {
  const envelope = await apiGet<SeriesRaw>('/api/areas/{id}/series', {
    path: { id: areaId },
    query: { version },
  });
  return adaptSeries(envelope);
}

/** События приходят конвертом с result_version — нужен для сборки bundle одной версии. */
interface EventsEnvelope {
  resultVersion: string | null;
  events: AnalysisEvent[];
}

async function fetchEvents(areaId: string, version?: string): Promise<EventsEnvelope> {
  const raw = await apiGet<{ result_version?: string; events?: EventRaw[] }>(
    '/api/areas/{id}/events',
    {
      path: { id: areaId },
      query: { version },
    },
  );
  return {
    resultVersion: raw.result_version ?? null,
    events: adaptEventList(raw),
  };
}

export function useAreas() {
  return useQuery({ queryKey: ['areas'], queryFn: fetchAreas });
}

export function useArea(areaId: string | null | undefined) {
  return useQuery({
    queryKey: ['area', areaId],
    queryFn: () => fetchArea(areaId as string),
    enabled: !!areaId,
  });
}

export function useContours(bbox: string | null | undefined, mockCase?: string) {
  return useQuery({
    queryKey: ['contours', bbox, mockCase ?? null],
    queryFn: () => fetchContours(bbox as string, mockCase),
    enabled: !!bbox,
    placeholderData: keepPreviousData,
  });
}

/**
 * Прогресс задачи: опрос каждые 2 с, пока queued|running; при сетевых сбоях —
 * backoff до 10 с, состояние connectionLost, задача не пересоздаётся.
 */
export function useJob(jobId: string | null | undefined) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['job', jobId],
    queryFn: () => fetchJob(jobId as string),
    enabled: !!jobId,
    refetchInterval: (q) => {
      const job = q.state.data;
      if (job && (job.status === 'queued' || job.status === 'running')) return JOB_POLL_MS;
      const error = q.state.error;
      if (jobId && isAppError(error) && error.status === 0) {
        return JOB_BACKOFF_MAX_MS;
      }
      return false;
    },
    // Продолжаем читать ту же задачу: backoff 1→2→4→8→10 с, новой задачи не создаём
    retry: (failureCount) => failureCount < 5,
    retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, JOB_BACKOFF_MAX_MS),
  });

  const job = query.data;
  const active = !!job && (job.status === 'queued' || job.status === 'running');

  // Терминальный статус → перечитать участок (новый activeJob/lastResult с result_version)
  useEffect(() => {
    if (
      job &&
      (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled')
    ) {
      void queryClient.invalidateQueries({ queryKey: ['area', job.areaId] });
      void queryClient.invalidateQueries({ queryKey: ['areas'] });
    }
  }, [job, queryClient]);

  return {
    job,
    isLoading: query.isPending && !!jobId,
    /** Сетевые сбои при активной задаче: показываем «Нет связи…», опрос продолжается. */
    connectionLost: query.failureCount > 0 && (active || !job),
  };
}

export interface BundleResult {
  /** meta может отсутствовать, пока вопрос о её источнике открыт (ui-spec §8.2). */
  meta?: Area['lastResult'];
  series: Series;
  events: AnalysisEvent[];
}

/**
 * Атомарный bundle одной версии (frontend-plan §4, бриф §6):
 * данные отдаются только когда series и events совпали по resultVersion;
 * иначе — предыдущий bundle целиком (placeholderData) или isLoading.
 */
export function useResultBundle(areaId: string | null | undefined, version?: string | null) {
  const seriesQuery = useQuery({
    queryKey: ['series', areaId, version ?? null],
    queryFn: () => fetchSeries(areaId as string, version ?? undefined),
    enabled: !!areaId,
    placeholderData: keepPreviousData,
  });
  const eventsQuery = useQuery({
    queryKey: ['events', areaId, version ?? null],
    queryFn: () => fetchEvents(areaId as string, version ?? undefined),
    enabled: !!areaId,
    placeholderData: keepPreviousData,
  });
  const areaQuery = useArea(areaId);

  const series = seriesQuery.data;
  const eventsEnvelope = eventsQuery.data;
  const matched =
    !!series &&
    !!eventsEnvelope &&
    eventsEnvelope.resultVersion !== null &&
    series.resultVersion === eventsEnvelope.resultVersion &&
    (!version || series.resultVersion === version);

  const meta = areaQuery.data?.lastResult;
  const metaMatched = matched && meta?.resultVersion === series?.resultVersion;

  return {
    bundle: matched
      ? {
          meta: metaMatched ? meta : undefined,
          series,
          events: eventsEnvelope.events,
        }
      : undefined,
    isLoading: seriesQuery.isPending || eventsQuery.isPending,
    versionMismatch: !matched && !!(series || eventsEnvelope),
  };
}

/**
 * Лимиты формы (corrections §1): null = не пришли → числовая валидация отключена.
 * Отсутствие /api/config (404) — штатный режим до реализации B3, не ошибка UI.
 */
export function useLimits() {
  return useQuery({
    queryKey: ['limits'],
    queryFn: async (): Promise<Limits | null> => {
      try {
        const raw = await apiGet<LimitsRaw>('/api/config');
        return adaptLimits(raw);
      } catch (error) {
        // 422 с limits в теле — второй согласованный способ доставки лимитов
        if (isAppError(error) && error.extra) {
          const fromError = adaptLimitsFromErrorBody(error.extra);
          if (fromError) return fromError;
        }
        if (isAppError(error) && error.status === 404) return null;
        throw error;
      }
    },
    staleTime: Number.POSITIVE_INFINITY,
  });
}

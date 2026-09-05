import { http, type HttpHandler, HttpResponse } from 'msw';

import type { AreaRaw } from '@/api/adapters/areas';
import type { JobRaw } from '@/api/adapters/jobs';
import type { SeriesRaw } from '@/api/adapters/series';
import areasFixture from './fixtures/areas.json';
import contoursEmpty from './fixtures/contours-empty.json';
import contoursFailed from './fixtures/contours-failed.json';
import contoursOk from './fixtures/contours-ok.json';
import eventsCandidate from './fixtures/events-candidate.json';
import eventsConfirmed from './fixtures/events-confirmed.json';
import eventsInsufficient from './fixtures/events-insufficient.json';
import eventsNormal from './fixtures/events-normal.json';
import jobsFixture from './fixtures/jobs.json';
import seriesCandidate from './fixtures/series-candidate.json';
import seriesConfirmed from './fixtures/series-confirmed.json';
import seriesInsufficient from './fixtures/series-insufficient.json';
import seriesNormal from './fixtures/series-normal.json';

/**
 * MSW-обработчики всех маршрутов таблицы §4 (frontend-plan).
 * Все ответы — фикстуры с _synthetic: true. Живой сценарий запуска:
 * стадии satellite → weather → prepare → analysis по 1.5 с, затем completed
 * с новой result_version (уточнение FE-1).
 */

// Список участков мутабельный: POST/DELETE и завершение live-задачи меняют его,
// чтобы инвалидация ['area', id] возвращала актуальные last_result/active_job.
const AREAS = [...(areasFixture.areas as AreaRaw[])];

const JOBS = jobsFixture.jobs as JobRaw[];

const SERIES: Record<string, SeriesRaw> = {
  'area-normal': (seriesNormal as { series: SeriesRaw }).series,
  'area-candidate': (seriesCandidate as { series: SeriesRaw }).series,
  'area-confirmed': (seriesConfirmed as { series: SeriesRaw }).series,
  'area-insufficient': (seriesInsufficient as { series: SeriesRaw }).series,
};

// Версия событий фикстуры = result_version в last_result участка — иначе bundle не соберётся
const EVENTS: Record<string, { version: string; events: unknown[] }> = {
  'area-normal': { version: 'v-normal-1', events: eventsNormal.events },
  'area-candidate': { version: 'v-candidate-1', events: eventsCandidate.events },
  'area-confirmed': { version: 'v-confirmed-1', events: eventsConfirmed.events },
  'area-insufficient': { version: 'v-insufficient-1', events: eventsInsufficient.events },
};

const CONTOURS: Record<string, object> = {
  ok: contoursOk,
  empty: contoursEmpty,
  failed: contoursFailed,
};

/** Стадии live-сценария: 4 стадии по 1.5 c → completed на 6-й секунде. */
const LIVE_STAGE_MS = 1500;
const LIVE_TOTAL_MS = LIVE_STAGE_MS * 4;

interface LiveJob {
  areaId: string;
  createdAt: number;
  period: { from: string; to: string };
}

const liveJobs = new Map<string, LiveJob>();

function notFound(title: string) {
  return HttpResponse.json({ _synthetic: true, code: 'not_found', title }, { status: 404 });
}

/** Завершение live-задачи обновляет last_result участка — как это сделает реальный backend. */
function completeLiveJob(job: LiveJob, resultVersion: string): void {
  const area = AREAS.find((item) => item.id === job.areaId);
  if (!area) return;
  const computedAt = new Date().toISOString();
  // у нового участка результата ещё нет — фиксируем «первичный» разбор (как ML вернёт)
  area.last_result = {
    ...(area.last_result ?? {
      verdict: 'candidate',
      severity: null,
      sources: { sentinel2: { status: 'ok' }, era5: { status: 'ok' } },
      limitations: [],
    }),
    result_version: resultVersion,
    period: { from: job.period.from, to: job.period.to },
    computed_at: computedAt,
  };
  area.active_job = null;
}

export const handlers: HttpHandler[] = [
  http.get('*/api/areas', () => HttpResponse.json({ _synthetic: true, areas: AREAS })),

  http.get('*/api/areas/:id', ({ params }) => {
    const area = AREAS.find((item) => item.id === params.id);
    if (!area) return notFound('Участок не найден');
    return HttpResponse.json({ _synthetic: true, ...area });
  }),

  http.post('*/api/areas', async ({ request }) => {
    const body = (await request.json()) as Partial<AreaRaw> | null;
    const area: AreaRaw = {
      id: `area-${Date.now()}`,
      name: body?.name ?? 'Новый участок',
      geometry: body?.geometry,
      // источник эхом из тела: контур каталога или нарисованный вручную
      source: {
        kind: body?.source?.kind === 'contour' ? 'contour' : 'drawn',
        label: body?.source?.label ?? 'Нарисован вручную',
        external_id: body?.source?.external_id,
      },
      created_at: new Date().toISOString(),
      last_result: null,
      active_job: null,
    };
    AREAS.push(area);
    return HttpResponse.json({ _synthetic: true, ...area }, { status: 201 });
  }),

  http.delete('*/api/areas/:id', ({ params }) => {
    const index = AREAS.findIndex((item) => item.id === params.id);
    if (index === -1) return notFound('Участок не найден');
    AREAS.splice(index, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  http.get('*/api/regions/contours', ({ request }) => {
    const url = new URL(request.url);
    // Переключение сценариев — query mock_case (dev-удобство поверх bbox)
    const mockCase = url.searchParams.get('mock_case') ?? url.searchParams.get('mockCase');
    const scenario = mockCase === 'empty' || mockCase === 'failed' ? mockCase : 'ok';
    return HttpResponse.json(CONTOURS[scenario]);
  }),

  http.post('*/api/areas/:id/analyses', async ({ request, params }) => {
    const areaId = String(params.id);
    // Фикстурный сценарий 429: очередь заполнена (corrections §1)
    if (areaId === 'area-normal') {
      await new Promise((resolveDelay) => setTimeout(resolveDelay, 150));
      return HttpResponse.json(
        { _synthetic: true, code: 'queue_full', title: 'Очередь анализа занята', status: 429 },
        { status: 429 },
      );
    }
    const body = (await request.json()) as { period?: { from: string; to: string } } | null;
    const jobId = `job-live-${Date.now()}`;
    liveJobs.set(jobId, {
      areaId,
      createdAt: Date.now(),
      period: body?.period ?? { from: '2024-03-01', to: '2024-09-30' },
    });
    const area = AREAS.find((item) => item.id === areaId);
    if (area) {
      area.active_job = {
        id: jobId,
        area_id: areaId,
        requested_period: body?.period ?? { from: '2024-03-01', to: '2024-09-30' },
        status: 'running',
        stage: 'satellite',
        message: null,
        error_code: null,
        error_message: null,
        result_version: null,
        updated_at: new Date().toISOString(),
      };
    }
    return HttpResponse.json({ _synthetic: true, job_id: jobId }, { status: 202 });
  }),

  http.get('*/api/jobs/:id', ({ params }) => {
    const id = String(params.id);
    const fixture = JOBS.find((item) => item.id === id);
    if (fixture) return HttpResponse.json({ _synthetic: true, ...fixture });

    const live = liveJobs.get(id);
    if (!live) return notFound('Задача не найдена');

    const elapsed = Date.now() - live.createdAt;
    const stage =
      elapsed < LIVE_STAGE_MS
        ? 'satellite'
        : elapsed < LIVE_STAGE_MS * 2
          ? 'weather'
          : elapsed < LIVE_STAGE_MS * 3
            ? 'prepare'
            : elapsed < LIVE_TOTAL_MS
              ? 'analysis'
              : null;
    const completed = elapsed >= LIVE_TOTAL_MS;
    const resultVersion = completed ? `v-live-${id}` : null;
    if (completed) completeLiveJob(live, resultVersion as string);

    return HttpResponse.json({
      _synthetic: true,
      id,
      area_id: live.areaId,
      requested_period: live.period,
      status: completed ? 'completed' : 'running',
      stage,
      message: null,
      error_code: null,
      error_message: null,
      result_version: resultVersion,
      updated_at: new Date().toISOString(),
    });
  }),

  http.get('*/api/areas/:id/series', ({ request, params }) => {
    const base = SERIES[String(params.id)];
    if (!base) return notFound('Ряд не найден');
    const version = new URL(request.url).searchParams.get('version');
    // version эхом возвращается: bundle соберётся при совпадении версий (вопрос B3 в §8.2)
    return HttpResponse.json({
      _synthetic: true,
      series: { ...base, result_version: version ?? base.result_version },
    });
  }),

  http.get('*/api/areas/:id/events', ({ request, params }) => {
    const base = EVENTS[String(params.id)];
    if (base === undefined) return notFound('События не найдены');
    const version = new URL(request.url).searchParams.get('version');
    // версия событий эхом возвращается, как у series: bundle собирается при совпадении
    return HttpResponse.json({
      _synthetic: true,
      result_version: version ?? base.version,
      events: base.events,
    });
  }),

  // Конфиг с лимитами командой ещё не отдан → 404, useLimits вернёт null
  http.get('*/api/config', () => notFound('Конфигурация не отдана')),
];

import { type AppError, isAppError } from '@/api/client';
import { useStartAnalysis } from '@/api/mutations';
import { useAreas, useJob, useResultBundle } from '@/api/queries';
import type { JobMeta, JobStatus, Verdict } from '@/api/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import {
  DEV_LABELS,
  EMPTY,
  EVENT_BLOCK_LABEL,
  JOB_LABEL,
  JOB_STAGE_FALLBACK,
  PROVENANCE_LABEL,
  STAGE_LABEL,
  VERDICT_LABEL,
} from '@/lib/labels';
import { type ReactNode, useEffect, useState } from 'react';
import { PreviewChart } from './PreviewChart';

/**
 * Dev-страница состояний (?dev=states, только import.meta.env.DEV — corrections §2).
 * Показывает фикстуры через РЕАЛЬНЫЕ хуки (useAreas/useResultBundle/useJob/
 * useStartAnalysis на MSW) — это проверка данных и контракта, не макет UI.
 * Страница техническая: таблицы в «сыром» виде, подписи — из labels.ts.
 */

const VERDICT_COLOR: Record<Verdict, string> = {
  normal: 'text-verdict-normal',
  candidate: 'text-verdict-candidate',
  confirmed: 'text-verdict-confirmed',
  insufficient_data: 'text-verdict-insufficient',
};

const JOB_COLOR: Record<JobStatus, string> = {
  queued: 'text-job-queued',
  running: 'text-job-running',
  completed: 'text-ink',
  failed: 'text-job-failed',
  cancelled: 'text-job-cancelled',
};

/** Статус/стадия задачи: только то, что реально передал backend (бриф §3B). */
function jobStatusText(job: JobMeta | undefined): { text: string; className: string } {
  if (!job) return { text: DEV_LABELS.dash, className: 'text-ink-tertiary' };
  if (job.status === 'running') {
    const stage = job.stage
      ? (STAGE_LABEL[job.stage as keyof typeof STAGE_LABEL] ?? JOB_STAGE_FALLBACK)
      : JOB_STAGE_FALLBACK;
    return { text: `${JOB_LABEL.running} · ${stage}`, className: JOB_COLOR.running };
  }
  if (job.status === 'queued') return { text: JOB_LABEL.queued, className: JOB_COLOR.queued };
  return { text: JOB_LABEL[job.status], className: JOB_COLOR[job.status] };
}

const Th = ({ children }: { children: ReactNode }) => (
  <th className="border border-border bg-surface-muted px-2 py-1 text-left font-medium">
    {children}
  </th>
);
const Td = ({ children, className }: { children: ReactNode; className?: string }) => (
  <td className={`border border-border px-2 py-1 align-top ${className ?? ''}`}>{children}</td>
);

export default function StatesPage() {
  const areasQuery = useAreas();
  const areas = areasQuery.data ?? [];
  const [selectedAreaId, setSelectedAreaId] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedAreaId && areas.length > 0) setSelectedAreaId(areas[0].id);
  }, [areas, selectedAreaId]);

  const selectedArea = areas.find((area) => area.id === selectedAreaId);
  const { bundle, isLoading } = useResultBundle(selectedAreaId);

  // Живой сценарий: запуск → 202 → стадии → completed (фикстуры MSW)
  const [liveJobId, setLiveJobId] = useState<string | null>(null);
  const [startError, setStartError] = useState<AppError | null>(null);
  const start = useStartAnalysis();
  const { job: liveJob, connectionLost } = useJob(liveJobId);

  const run = (areaId: string) => {
    setStartError(null);
    // Период — из данных участка, ничего не выдумываем
    const period = selectedArea?.lastResult?.period ?? { from: '2024-03-01', to: '2024-09-30' };
    start.mutate(
      { areaId, period },
      {
        onSuccess: ({ jobId }) => setLiveJobId(jobId),
        onError: (error) => setStartError(isAppError(error) ? error : null),
      },
    );
  };

  const liveStatus = jobStatusText(liveJob);

  return (
    <div data-testid="states-page" className="min-h-dvh space-y-8 overflow-x-hidden bg-app p-4">
      <header className="flex items-center gap-3">
        <h1 className="text-xl font-semibold">{DEV_LABELS.title}</h1>
        <span className="text-2xs text-ink-tertiary">
          {PROVENANCE_LABEL.missing}: {EMPTY.demo}
        </span>
      </header>

      {/* --- Таблица участков: три независимых поля — вывод, тяжесть, задача --- */}
      <section aria-label={DEV_LABELS.areasTable}>
        <h2 className="mb-2 text-lg font-semibold">{DEV_LABELS.areasTable}</h2>
        {areasQuery.isPending ? (
          <Skeleton className="h-32 w-full" />
        ) : (
          <table className="w-full border-collapse bg-surface text-2xs">
            <thead>
              <tr>
                <Th>{DEV_LABELS.selectArea}</Th>
                <Th>{DEV_LABELS.columnSource}</Th>
                <Th>{DEV_LABELS.columnVerdict}</Th>
                <Th>{DEV_LABELS.columnSeverity}</Th>
                <Th>{DEV_LABELS.columnPeriod}</Th>
                <Th>{DEV_LABELS.columnJob}</Th>
              </tr>
            </thead>
            <tbody>
              {areas.map((area) => {
                const status = jobStatusText(area.activeJob);
                return (
                  <tr
                    key={area.id}
                    className={area.id === selectedAreaId ? 'bg-action-soft' : undefined}
                  >
                    <Td>
                      <button
                        type="button"
                        className="tap-target -my-2 w-full px-1 py-2 text-left text-2xs hover:underline"
                        onClick={() => setSelectedAreaId(area.id)}
                      >
                        {area.name}
                      </button>
                    </Td>
                    <Td>{area.source.label}</Td>
                    <Td>
                      {area.lastResult ? (
                        <span className={VERDICT_COLOR[area.lastResult.verdict]}>
                          {VERDICT_LABEL[area.lastResult.verdict]}
                        </span>
                      ) : (
                        DEV_LABELS.dash
                      )}
                    </Td>
                    {/* Тяжесть — отдельная колонка, отдельное поле */}
                    <Td>{area.lastResult?.severity ?? DEV_LABELS.dash}</Td>
                    <Td>
                      {area.lastResult
                        ? `${area.lastResult.period.from} — ${area.lastResult.period.to}`
                        : DEV_LABELS.dash}
                    </Td>
                    <Td className={status.className}>{status.text}</Td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </section>

      {/* --- Точки ряда выбранного участка --- */}
      <section aria-label={DEV_LABELS.seriesTable}>
        <h2 className="mb-2 text-lg font-semibold">{DEV_LABELS.seriesTable}</h2>
        <div className="mb-2 flex items-center gap-2">
          <span className="text-2xs text-ink-secondary">{DEV_LABELS.selectArea}:</span>
          <Select value={selectedAreaId ?? undefined} onValueChange={setSelectedAreaId}>
            <SelectTrigger className="w-64" aria-label={DEV_LABELS.selectArea}>
              <SelectValue placeholder={DEV_LABELS.dash} />
            </SelectTrigger>
            <SelectContent>
              {areas.map((area) => (
                <SelectItem key={area.id} value={area.id} className="text-2xs">
                  {area.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {isLoading && <Skeleton className="h-24 w-full" />}
        {bundle && (
          <>
            <p className="mb-1 text-2xs text-ink-tertiary">
              result_version: {bundle.series.resultVersion}
              {bundle.series.background ? ` · ${bundle.series.background.label}` : ''}
            </p>
            <PreviewChart series={bundle.series} />
            <div className="mt-2 max-h-80 overflow-y-auto">
              <table className="w-full border-collapse bg-surface text-2xs">
                <thead>
                  <tr>
                    <Th>{DEV_LABELS.columnDate}</Th>
                    <Th>{DEV_LABELS.columnNdvi}</Th>
                    <Th>{DEV_LABELS.columnProvenance}</Th>
                    <Th>{DEV_LABELS.columnBackground}</Th>
                    <Th>{DEV_LABELS.columnDeviation}</Th>
                  </tr>
                </thead>
                <tbody>
                  {bundle.series.points.map((point) => (
                    <tr key={point.date}>
                      <Td>{point.date}</Td>
                      {/* null → «Нет данных», никогда не 0 */}
                      <Td>
                        {point.ndvi === null ? PROVENANCE_LABEL.missing : point.ndvi.toFixed(2)}
                      </Td>
                      <Td>{PROVENANCE_LABEL[point.provenance]}</Td>
                      <Td>
                        {point.background
                          ? point.background.mean.toFixed(2)
                          : PROVENANCE_LABEL.missing}
                      </Td>
                      <Td>
                        {point.deviation
                          ? `${point.deviation.value} ${point.deviation.unit} к ${point.deviation.base || DEV_LABELS.dash}`
                          : DEV_LABELS.dash}
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>

      {/* --- События: четыре блока в сыром виде --- */}
      <section aria-label={DEV_LABELS.eventsList}>
        <h2 className="mb-2 text-lg font-semibold">{DEV_LABELS.eventsList}</h2>
        {bundle && bundle.events.length > 0 ? (
          <div className="space-y-3">
            {bundle.events.map((event) => (
              <article
                key={event.id}
                className="rounded-md border border-border bg-surface p-3 text-2xs"
              >
                <div className="mb-2 flex items-center gap-2">
                  <Badge variant="outline" className={VERDICT_COLOR[event.verdict]}>
                    {VERDICT_LABEL[event.verdict]}
                  </Badge>
                  {event.severity && <Badge variant="secondary">{event.severity}</Badge>}
                  <span className="text-ink-tertiary">
                    {event.period.from} — {event.period.to}
                  </span>
                </div>
                <dl className="grid gap-2">
                  <div>
                    <dt className="font-semibold">{EVENT_BLOCK_LABEL.detected}</dt>
                    <dd>
                      {event.detected.text} ({event.detected.magnitude} {event.detected.unit} к{' '}
                      {event.detected.base})
                    </dd>
                  </div>
                  <div>
                    <dt className="font-semibold">{EVENT_BLOCK_LABEL.basis}</dt>
                    <dd>
                      наблюдений: {event.basis.observedCount}, восстановлено:{' '}
                      {event.basis.imputedCount}, фон сопоставим:{' '}
                      {event.basis.backgroundComparable ? 'да' : 'нет'}
                      {event.basis.gapsNote ? `, ${event.basis.gapsNote}` : ''}
                      {event.basis.criteria ? `, критерии: ${event.basis.criteria}` : ''}
                    </dd>
                  </div>
                  <div>
                    <dt className="font-semibold">{EVENT_BLOCK_LABEL.weather}</dt>
                    <dd>
                      {event.weather ? (
                        <>
                          <ul className="list-disc pl-4">
                            {event.weather.facts.map((fact) => (
                              <li key={fact.label}>
                                {fact.label}: {fact.value}
                              </li>
                            ))}
                          </ul>
                          {/* гипотеза отдельна от фактов; null → строка из брифа */}
                          <p className="mt-1">{event.weather.hypothesis ?? EMPTY.causeUnknown}</p>
                        </>
                      ) : (
                        PROVENANCE_LABEL.missing
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt className="font-semibold">{EVENT_BLOCK_LABEL.limitations}</dt>
                    <dd>
                      {event.limitations.length > 0 ? (
                        <ul className="list-disc pl-4">
                          {event.limitations.map((limitation) => (
                            <li key={limitation}>{limitation}</li>
                          ))}
                        </ul>
                      ) : (
                        DEV_LABELS.dash
                      )}
                    </dd>
                  </div>
                </dl>
              </article>
            ))}
          </div>
        ) : (
          <p className="text-2xs text-ink-secondary">
            {bundle ? EMPTY.causeUnknown : PROVENANCE_LABEL.missing}
          </p>
        )}
      </section>

      {/* --- Живой сценарий задачи --- */}
      <section aria-label={DEV_LABELS.jobBlock}>
        <h2 className="mb-2 text-lg font-semibold">{DEV_LABELS.jobBlock}</h2>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            disabled={start.isPending}
            onClick={() => selectedAreaId && run(selectedAreaId)}
          >
            {DEV_LABELS.runAnalysis}
          </Button>
          {/* Отдельная кнопка 429: у area-normal POST отвечает queue_full */}
          <Button
            size="sm"
            variant="outline"
            disabled={start.isPending}
            onClick={() => run('area-normal')}
          >
            {DEV_LABELS.runQueueFull}
          </Button>
        </div>
        {start.isPending && <p className="mt-2 text-2xs text-ink-secondary">…</p>}
        {startError && (
          <p className="mt-2 text-2xs text-verdict-confirmed">
            {DEV_LABELS.error}: {startError.code} ({startError.title})
          </p>
        )}
        {liveJob && (
          <dl className="mt-3 grid max-w-md grid-cols-2 gap-x-4 gap-y-1 text-2xs">
            <dt className="text-ink-tertiary">job_id</dt>
            <dd>{liveJob.id}</dd>
            <dt className="text-ink-tertiary">{DEV_LABELS.columnJob}</dt>
            <dd className={liveStatus.className}>{liveStatus.text}</dd>
            <dt className="text-ink-tertiary">result_version</dt>
            <dd>{liveJob.resultVersion ?? PROVENANCE_LABEL.missing}</dd>
          </dl>
        )}
        {connectionLost && (
          <p className="mt-2 text-2xs text-verdict-candidate">{EMPTY.connectionLost}</p>
        )}
      </section>
    </div>
  );
}

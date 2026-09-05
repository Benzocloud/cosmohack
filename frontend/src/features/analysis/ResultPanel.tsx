import type { AnalysisEvent, Area, Series } from '@/api/types';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PreviewChart } from '@/features/dev/PreviewChart';
import { jobStatusText } from '@/lib/job-status';
import { EMPTY, EVENT_BLOCK_LABEL, PROVENANCE_LABEL, VERDICT_LABEL } from '@/lib/labels';

interface ResultData {
  bundle?: { meta?: Area['lastResult']; series: Series; events: AnalysisEvent[] };
  isLoading: boolean;
  versionMismatch: boolean;
  error?: unknown;
  refetch: () => Promise<unknown>;
}

export interface ResultViewProps {
  area?: Area;
  areaLoading: boolean;
  areaError?: unknown;
  result: ResultData;
  onRetryArea?: () => void;
  onRetryResult?: () => void;
}

function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <Alert variant="destructive" className="m-4">
      <AlertTitle>{message}</AlertTitle>
      {onRetry && (
        <AlertDescription>
          <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
            Повторить
          </Button>
        </AlertDescription>
      )}
    </Alert>
  );
}

function ResultLoading() {
  return (
    <div className="space-y-3 p-5" aria-label="Загрузка результата">
      <Skeleton className="h-5 w-2/3" />
      <Skeleton className="h-4 w-1/2" />
      <Skeleton className="h-20 w-full" />
    </div>
  );
}

function EmptyResult() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center p-8 text-center">
      <p className="text-sm font-medium text-ink">{EMPTY.resultNotReady}</p>
      <p className="mt-2 max-w-[260px] text-xs leading-relaxed text-ink-tertiary">
        Запустите анализ выбранного участка, чтобы здесь появились показатели и события.
      </p>
    </div>
  );
}

function methodLabel(method?: string) {
  switch (method) {
    case 'gradient_boosting_residual':
      return 'Градиентный бустинг (HGB)';
    case 'gaussian_smoothing_h8':
      return 'Сглаживание ряда';
    case 'nearest_neighbour_mean':
      return 'Соседнее значение';
    case 'no_estimate':
      return 'Оценка не построена';
    default:
      return method || 'Не определён';
  }
}

function resultLimitations(result: {
  limitations: string[];
  method?: string;
  insufficient: boolean;
}) {
  if (result.limitations.length > 0) return result.limitations;
  if (result.insufficient) {
    return ['Недостаточно пригодных наблюдений для надёжного вывода'];
  }
  if (result.method && result.method !== 'gradient_boosting_residual') {
    return ['Использован резервный метод восстановления'];
  }
  return [];
}

function AreaSummary({ area, result }: { area: Area; result: ResultData }) {
  const job = area.activeJob;
  const status = jobStatusText(job);
  const meta = result.bundle?.meta ?? area.lastResult;
  const limitations = meta
    ? resultLimitations({ ...meta, insufficient: meta.verdict === 'insufficient_data' })
    : [];

  return (
    <div className="space-y-4 p-5">
      <div>
        <p className="text-2xs font-medium uppercase tracking-[0.12em] text-ink-tertiary">
          Карточка участка
        </p>
        <h2 className="mt-1 text-base font-semibold tracking-[-0.01em]">{area.name}</h2>
        <p className="mt-1 text-xs text-ink-secondary">{area.source.label}</p>
      </div>

      {job && (
        <div
          className="rounded-md border border-border bg-surface-muted p-3"
          aria-label="Состояние задачи"
        >
          <p className="text-2xs font-medium text-ink-tertiary">Анализ</p>
          <p className={`mt-1 text-sm ${status.className}`}>{status.text}</p>
          {job.message && <p className="mt-1 text-xs text-ink-secondary">{job.message}</p>}
          {job.errorMessage && (
            <p className="mt-1 text-xs text-verdict-confirmed">{job.errorMessage}</p>
          )}
        </div>
      )}

      {meta && (
        <div className="space-y-3">
          <div>
            <p className="text-2xs text-ink-tertiary">Вывод</p>
            <p className="mt-1 text-sm font-medium">{VERDICT_LABEL[meta.verdict]}</p>
            {meta.severity && (
              <Badge variant="secondary" className="mt-2">
                {meta.severity}
              </Badge>
            )}
          </div>
          <div className="grid grid-cols-2 gap-3 text-xs">
            <div>
              <p className="text-2xs text-ink-tertiary">Период</p>
              <p className="mt-1 text-ink-secondary">
                {meta.period.from} — {meta.period.to}
              </p>
            </div>
            <div>
              <p className="text-2xs text-ink-tertiary">Результат</p>
              <p className="mt-1 truncate text-ink-secondary" title={meta.resultVersion}>
                {meta.resultVersion}
              </p>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3 text-xs">
            <div>
              <p className="text-2xs text-ink-tertiary">Метод</p>
              <p className="mt-1 text-ink-secondary">{methodLabel(meta.method)}</p>
            </div>
            <div>
              <p className="text-2xs text-ink-tertiary">Пригодные наблюдения</p>
              <p className="mt-1 text-ink-secondary">{meta.usableCount ?? '—'}</p>
            </div>
          </div>
          {(meta.imputedCount !== undefined || meta.missingCount !== undefined) && (
            <p className="text-xs text-ink-secondary">
              Восстановлено: {meta.imputedCount ?? 0} · без значения: {meta.missingCount ?? 0}
            </p>
          )}
          {Object.keys(meta.sources).length > 0 && (
            <div>
              <p className="text-2xs text-ink-tertiary">Источники</p>
              <ul className="mt-1 space-y-1 text-xs text-ink-secondary">
                {Object.entries(meta.sources).map(([source, value]) => (
                  <li key={source} className="flex justify-between gap-2">
                    <span>{source}</span>
                    <span>{value.note ?? value.status}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {limitations.length > 0 && (
            <div>
              <p className="text-2xs text-ink-tertiary">Ограничения</p>
              <ul className="mt-1 list-disc space-y-1 pl-4 text-xs text-ink-secondary">
                {limitations.map((limitation) => (
                  <li key={limitation}>{limitation}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function AreaCard({
  area,
  areaLoading,
  areaError,
  result,
  onRetryArea,
  onRetryResult,
}: ResultViewProps) {
  if (!area) {
    if (areaLoading) return <ResultLoading />;
    if (areaError) return <ErrorState message={EMPTY.resultLoadFailed} onRetry={onRetryArea} />;
    return <EmptyResult />;
  }
  return (
    <section aria-label="Карточка участка" className="h-full overflow-y-auto">
      <AreaSummary area={area} result={result} />
      {area.lastResult && result.error ? (
        <ErrorState message={EMPTY.resultLoadFailed} onRetry={onRetryResult} />
      ) : area.lastResult && result.versionMismatch ? (
        <ErrorState message={EMPTY.resultVersionMismatch} onRetry={onRetryResult} />
      ) : area.lastResult && result.isLoading && !result.bundle ? (
        <ResultLoading />
      ) : null}
    </section>
  );
}

function WeatherSummary({ series }: { series: Series }) {
  const weather = series.weather;
  if (!weather) {
    return <p className="text-xs text-ink-tertiary">{EMPTY.weatherPartial}</p>;
  }
  const temperature = [...weather.temperature].reverse().find((point) => point.value !== null);
  const precipitation = [...weather.precipitation].reverse().find((point) => point.value !== null);
  return (
    <div className="grid grid-cols-2 gap-3 text-xs">
      <div className="rounded-md border border-border p-3">
        <p className="text-2xs text-ink-tertiary">Температура</p>
        <p className="mt-1 font-medium">
          {temperature?.value === undefined || temperature.value === null
            ? PROVENANCE_LABEL.missing
            : `${temperature.value.toFixed(1)} ${weather.units.temperature}`}
        </p>
      </div>
      <div className="rounded-md border border-border p-3">
        <p className="text-2xs text-ink-tertiary">Осадки</p>
        <p className="mt-1 font-medium">
          {precipitation?.value === undefined || precipitation.value === null
            ? PROVENANCE_LABEL.missing
            : `${precipitation.value.toFixed(1)} ${weather.units.precipitation}`}
        </p>
      </div>
      <p className="col-span-2 text-2xs text-ink-tertiary">
        {weather.source || 'Источник погоды'} · покрытие {weather.coverage.from} —{' '}
        {weather.coverage.to}
      </p>
    </div>
  );
}

function EventCard({ event }: { event: AnalysisEvent }) {
  return (
    <article className="rounded-md border border-border p-3 text-xs">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline">{VERDICT_LABEL[event.verdict]}</Badge>
        {event.severity && <Badge variant="secondary">{event.severity}</Badge>}
        <span className="text-2xs text-ink-tertiary">
          {event.period.from} — {event.period.to}
        </span>
      </div>
      <dl className="mt-3 space-y-2">
        <div>
          <dt className="text-2xs font-medium text-ink-tertiary">{EVENT_BLOCK_LABEL.detected}</dt>
          <dd>{event.detected.text || PROVENANCE_LABEL.missing}</dd>
        </div>
        <div>
          <dt className="text-2xs font-medium text-ink-tertiary">{EVENT_BLOCK_LABEL.basis}</dt>
          <dd>
            {event.basis.observedCount} наблюдений, {event.basis.imputedCount} восстановлено
          </dd>
        </div>
        {event.weather && (
          <div>
            <dt className="text-2xs font-medium text-ink-tertiary">{EVENT_BLOCK_LABEL.weather}</dt>
            <dd>{event.weather.facts.map((fact) => `${fact.label}: ${fact.value}`).join('; ')}</dd>
            {event.weather.hypothesis && (
              <dd className="mt-1 text-ink-secondary">{event.weather.hypothesis}</dd>
            )}
          </div>
        )}
        {event.limitations.length > 0 && (
          <div>
            <dt className="text-2xs font-medium text-ink-tertiary">
              {EVENT_BLOCK_LABEL.limitations}
            </dt>
            <dd>{event.limitations.join('; ')}</dd>
          </div>
        )}
      </dl>
    </article>
  );
}

export function AnalysisTimeline({
  area,
  areaLoading,
  areaError,
  result,
  onRetryArea,
  onRetryResult,
}: ResultViewProps) {
  if (!area) {
    if (areaLoading) return <ResultLoading />;
    if (areaError) return <ErrorState message={EMPTY.resultLoadFailed} onRetry={onRetryArea} />;
    return <EmptyResult />;
  }
  if (!area.lastResult) return <EmptyResult />;
  if (result.error) return <ErrorState message={EMPTY.resultLoadFailed} onRetry={onRetryResult} />;
  if (result.versionMismatch)
    return <ErrorState message={EMPTY.resultVersionMismatch} onRetry={onRetryResult} />;
  if (result.isLoading || !result.bundle) return <ResultLoading />;

  const { series, events } = result.bundle;
  const limitations = resultLimitations({
    ...series,
    insufficient: series.status === 'insufficient_data',
  });
  return (
    <div className="h-full min-h-0 overflow-y-auto p-4">
      <div className="rounded-md border border-border bg-surface">
        <div className="border-b border-border px-4 py-3">
          <h3 className="text-sm font-medium">Динамика NDVI</h3>
          <p className="mt-1 text-2xs text-ink-tertiary">
            {series.period.from} — {series.period.to} · {series.points.length} наблюдений
          </p>
          <div className="mt-3 grid grid-cols-2 gap-2 text-2xs text-ink-secondary sm:grid-cols-4">
            <div>
              <p className="text-ink-tertiary">Метод</p>
              <p className="mt-0.5 font-medium text-ink">{methodLabel(series.method)}</p>
            </div>
            <div>
              <p className="text-ink-tertiary">Пригодные</p>
              <p className="mt-0.5 font-medium text-ink">{series.usableCount}</p>
            </div>
            <div>
              <p className="text-ink-tertiary">Восстановлено</p>
              <p className="mt-0.5 font-medium text-ink">{series.imputedCount}</p>
            </div>
            <div>
              <p className="text-ink-tertiary">Без значения</p>
              <p className="mt-0.5 font-medium text-ink">{series.missingCount}</p>
            </div>
          </div>
          {limitations.length > 0 && (
            <Alert className="mt-3 border-amber-200 bg-amber-50 py-2">
              <AlertTitle className="text-xs">Ограничения результата</AlertTitle>
              <AlertDescription className="text-2xs text-ink-secondary">
                {limitations.join(' · ')}
              </AlertDescription>
            </Alert>
          )}
        </div>
        {series.points.length > 0 ? (
          <PreviewChart series={series} />
        ) : (
          <p className="p-6 text-center text-xs text-ink-tertiary">{EMPTY.noSatellite}</p>
        )}
      </div>
      <div className="mt-3 rounded-md border border-border bg-surface p-4">
        <h3 className="text-sm font-medium">Погодный контекст</h3>
        <div className="mt-3">
          <WeatherSummary series={series} />
        </div>
      </div>
      <div className="mt-3 rounded-md border border-border bg-surface p-4">
        <h3 className="text-sm font-medium">События</h3>
        <div className="mt-3 space-y-2">
          {events.length > 0 ? (
            events.map((event) => <EventCard key={event.id} event={event} />)
          ) : (
            <p className="text-xs text-ink-tertiary">{VERDICT_LABEL.normal}</p>
          )}
        </div>
      </div>
    </div>
  );
}

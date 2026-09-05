import { useStartAnalysis } from '@/api/mutations';
import { useArea, useJob, useLimits } from '@/api/queries';
import type { Period } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { validatePeriod } from '@/lib/geo';
import { jobStatusText } from '@/lib/job-status';
import { SCAFFOLD } from '@/lib/labels';
import { useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';
import { CalendarDays, Sprout } from 'lucide-react';
import { useEffect, useState } from 'react';
import { DemoBadge } from './DemoBadge';

/**
 * Шапка 56px на десктопе/планшете, 48px на телефоне (frontend-plan §6.1, §6.3).
 * Период и запуск анализа используют тот же API, что и диалог добавления участка.
 */
export function Header() {
  const demoMode = useUi((s) => s.demoMode);
  const selectedAreaId = useSelection((s) => s.selectedAreaId);
  const areaQuery = useArea(selectedAreaId);
  const limitsQuery = useLimits();
  const start = useStartAnalysis();
  const [period, setPeriod] = useState<Period | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const trackedJobId = jobId ?? areaQuery.data?.activeJob?.id ?? null;
  const { job } = useJob(trackedJobId);

  useEffect(() => {
    if (selectedAreaId !== areaQuery.data?.id) {
      setPeriod(null);
      setJobId(null);
    }
  }, [selectedAreaId, areaQuery.data?.id]);

  useEffect(() => {
    if (!areaQuery.data) return;
    setPeriod(areaQuery.data.period ?? areaQuery.data.lastResult?.period ?? null);
  }, [areaQuery.data]);

  const selectedPeriod = period ?? areaQuery.data?.period ?? areaQuery.data?.lastResult?.period;
  const periodValidation = selectedPeriod
    ? validatePeriod(selectedPeriod.from, selectedPeriod.to, limitsQuery.data ?? null)
    : { ok: false };
  const canRun = Boolean(
    selectedAreaId && selectedPeriod?.from && selectedPeriod.to && periodValidation.ok,
  );
  const periodError =
    selectedAreaId && selectedPeriod?.from && selectedPeriod.to && !periodValidation.ok
      ? periodValidation.error
      : null;
  const status = jobStatusText(job);

  const updatePeriod = (field: keyof Period, value: string) => {
    setPeriod((current) => ({
      from: current?.from ?? selectedPeriod?.from ?? '',
      to: current?.to ?? selectedPeriod?.to ?? '',
      [field]: value,
    }));
  };

  const runAnalysis = () => {
    if (!selectedAreaId || !selectedPeriod || !canRun || !periodValidation.ok || start.isPending)
      return;
    setJobId(null);
    start.mutate(
      { areaId: selectedAreaId, period: selectedPeriod },
      { onSuccess: ({ jobId: nextJobId }) => setJobId(nextJobId) },
    );
  };

  return (
    <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border bg-surface px-3 shadow-[0_1px_0_rgba(21,26,33,.03)] sm:px-5 lg:h-14">
      <div className="flex items-center gap-2">
        <div
          aria-hidden="true"
          className="flex size-8 shrink-0 items-center justify-center rounded-md bg-action text-white shadow-1"
        >
          <Sprout className="size-4" />
        </div>
        <div className="flex flex-col">
          <span className="text-sm font-semibold leading-tight tracking-[-0.01em]">
            {SCAFFOLD.appTitle}
          </span>
          <span className="hidden text-2xs leading-tight text-ink-tertiary sm:block">
            {SCAFFOLD.appSubtitle}
          </span>
        </div>
      </div>

      <div className="mx-auto hidden items-center gap-3 rounded-md bg-app px-3 py-1.5 lg:flex">
        <span className="text-sm text-ink-secondary">
          {selectedAreaId ?? SCAFFOLD.noAreaSelected}
        </span>
        <div className="flex items-center gap-1 text-2xs text-ink-tertiary">
          <CalendarDays aria-hidden />
          <span className="sr-only">Период анализа, с</span>
          <Input
            type="date"
            value={selectedPeriod?.from ?? ''}
            min={limitsQuery.data?.minDate}
            onChange={(event) => updatePeriod('from', event.target.value)}
            aria-label="Период анализа с"
            className="h-8 w-[124px] bg-surface text-2xs"
            disabled={!selectedAreaId}
          />
          <span>—</span>
          <span className="sr-only">Период анализа, по</span>
          <Input
            type="date"
            value={selectedPeriod?.to ?? ''}
            min={limitsQuery.data?.minDate}
            onChange={(event) => updatePeriod('to', event.target.value)}
            aria-label="Период анализа по"
            className="h-8 w-[124px] bg-surface text-2xs"
            disabled={!selectedAreaId}
          />
        </div>
      </div>

      <div className="ml-auto flex items-center gap-2">
        <DemoBadge demo={demoMode} />
        <Button
          size="sm"
          disabled={
            !canRun ||
            start.isPending ||
            Boolean(job && (job.status === 'queued' || job.status === 'running'))
          }
          onClick={runAnalysis}
          className="inline-flex"
        >
          {start.isPending
            ? 'Запуск…'
            : job && (job.status === 'queued' || job.status === 'running')
              ? status.text
              : SCAFFOLD.runAnalysis}
        </Button>
      </div>
      {(periodError || start.isError) && (
        <span
          role="alert"
          className="absolute right-3 top-full z-30 mt-1 rounded-md bg-surface px-3 py-2 text-2xs text-verdict-confirmed shadow-2"
        >
          {periodError ?? 'Не удалось запустить анализ. Повторите попытку.'}
        </span>
      )}
    </header>
  );
}

import { useStartAnalysis } from '@/api/mutations';
import { useAreas, useJob } from '@/api/queries';
import type { Verdict } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { jobStatusText } from '@/lib/job-status';
import { AREA_LIST_LABELS, VERDICT_LABEL } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { selectionActions, useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';

/**
 * МИНИМАЛЬНЫЙ список участков (уточнение FE-3): имя + verdict/«Не анализировался»
 * + состояние задачи + выделение выбранного — проверка связи список ↔ карта.
 * Полноценные AreaListItem/AreaCard реализуются на FE-2.
 * Строка с неудавшимся запуском получает отдельную кнопку «Запустить анализ» (§3A).
 */

const VERDICT_COLOR: Record<Verdict, string> = {
  normal: 'text-verdict-normal',
  candidate: 'text-verdict-candidate',
  confirmed: 'text-verdict-confirmed',
  insufficient_data: 'text-verdict-insufficient',
};

export function AreaList() {
  const areasQuery = useAreas();
  const areas = areasQuery.data ?? [];
  const selectedAreaId = useSelection((s) => s.selectedAreaId);
  const pendingStart = useUi((s) => s.pendingStart);
  const setPendingStart = useUi((s) => s.setPendingStart);
  const start = useStartAnalysis();

  if (areasQuery.isPending) {
    return (
      <div className="flex flex-col gap-2 p-4">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  return (
    <ul className="flex flex-col">
      {areas.map((area) => {
        const selected = area.id === selectedAreaId;
        const status = jobStatusText(area.activeJob);
        const needsManualStart = pendingStart?.areaId === area.id && !area.activeJob;
        return (
          <li
            key={area.id}
            className={cn(
              'flex items-stretch border-b border-border last:border-b-0',
              selected ? 'bg-action-soft' : 'hover:bg-surface-hover',
            )}
          >
            <button
              type="button"
              onClick={() => selectionActions.selectArea(area.id, 'list')}
              aria-current={selected ? 'true' : undefined}
              className="flex min-h-11 min-w-0 flex-1 flex-col items-start gap-0.5 px-4 py-2 text-left"
            >
              <span className="truncate text-sm font-medium">{area.name}</span>
              {area.lastResult ? (
                <span className={cn('text-2xs', VERDICT_COLOR[area.lastResult.verdict])}>
                  {VERDICT_LABEL[area.lastResult.verdict]}
                </span>
              ) : (
                <span className="text-2xs text-ink-tertiary">{AREA_LIST_LABELS.notAnalyzed}</span>
              )}
              {area.activeJob && (
                <span className={cn('text-2xs', status.className)}>{status.text}</span>
              )}
            </button>
            {area.activeJob && <JobWatcher jobId={area.activeJob.id} />}
            {needsManualStart && (
              <Button
                variant="link"
                size="sm"
                className="min-h-11 self-center px-2 text-2xs"
                disabled={start.isPending}
                onClick={() => {
                  if (!pendingStart) return;
                  start.mutate(
                    { areaId: pendingStart.areaId, period: pendingStart.period },
                    { onSuccess: () => setPendingStart(null) },
                  );
                }}
              >
                {AREA_LIST_LABELS.runAnalysis}
              </Button>
            )}
          </li>
        );
      })}
    </ul>
  );
}

function JobWatcher({ jobId }: { jobId: string }) {
  useJob(jobId);
  return null;
}

import { Button } from '@/components/ui/button';
import { SCAFFOLD } from '@/lib/labels';
import { useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';
import { CalendarDays, Sprout } from 'lucide-react';
import { DemoBadge } from './DemoBadge';

/**
 * Шапка 56px на десктопе/планшете, 48px на телефоне (frontend-plan §6.1, §6.3).
 * Период и запуск анализа — заглушки до FE-5 (PeriodPicker и сценарий повторного анализа).
 */
export function Header() {
  const demoMode = useUi((s) => s.demoMode);
  const selectedAreaId = useSelection((s) => s.selectedAreaId);

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
        {/* PeriodPicker подключается на FE-5; ограничения периода — только из useLimits */}
        <Button variant="outline" size="sm" disabled>
          <CalendarDays aria-hidden />
          {SCAFFOLD.periodNotSelected}
        </Button>
      </div>

      <div className="ml-auto flex items-center gap-2">
        <DemoBadge demo={demoMode} />
        <Button size="sm" disabled={Boolean(!selectedAreaId)} className="hidden sm:inline-flex">
          {SCAFFOLD.runAnalysis}
        </Button>
      </div>
    </header>
  );
}

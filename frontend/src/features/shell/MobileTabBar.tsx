import { SCAFFOLD } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { type MobileTab, useUi } from '@/store/ui';
import { LineChart, Map as MapIcon, Rows3 } from 'lucide-react';

const TABS: { id: MobileTab; label: string; Icon: typeof MapIcon }[] = [
  { id: 'areas', label: SCAFFOLD.mobileTabAreas, Icon: Rows3 },
  { id: 'map', label: SCAFFOLD.mobileTabMap, Icon: MapIcon },
  { id: 'analysis', label: SCAFFOLD.mobileTabAnalysis, Icon: LineChart },
];

/**
 * Нижняя навигация телефона 56px: Участки · Карта · Анализ (frontend-plan §6.3).
 * Выбор участка, события и даты сохраняется при переключении — они живут в URL/stores.
 */
export function MobileTabBar() {
  const mobileTab = useUi((s) => s.mobileTab);
  const setMobileTab = useUi((s) => s.setMobileTab);

  return (
    <nav
      aria-label={SCAFFOLD.mainNavigation}
      className="flex h-14 shrink-0 border-t border-border bg-surface lg:hidden"
    >
      {TABS.map(({ id, label, Icon }) => {
        const active = mobileTab === id;
        return (
          <button
            key={id}
            type="button"
            aria-label={label}
            aria-current={active ? 'page' : undefined}
            onClick={() => setMobileTab(id)}
            className={cn(
              'tap-target flex flex-1 flex-col items-center justify-center gap-0.5 text-2xs',
              active ? 'text-action' : 'text-ink-tertiary',
            )}
          >
            <Icon className="size-5" aria-hidden />
            {label}
          </button>
        );
      })}
    </nav>
  );
}

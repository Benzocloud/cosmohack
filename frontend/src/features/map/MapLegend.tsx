import { Button } from '@/components/ui/button';
import { MAP_LABELS, VERDICT_LABEL } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useState } from 'react';

/**
 * Легенда карты (план §6.4): статусы вывода, «не анализировался»,
 * «найденный контур», «выбранный». На мобиле сворачивается.
 */
export function MapLegend() {
  // Легенда не должна закрывать спутниковую подложку при первом открытии.
  // Пользователь раскрывает её явной кнопкой/стрелкой.
  const [expanded, setExpanded] = useState(false);

  return (
    <div
      className={cn(
        'absolute left-0 top-1/2 z-20 -translate-y-1/2 overflow-hidden rounded-r-md border border-l-0 border-slate-300 bg-white shadow-2 transition-[width] duration-200',
        expanded ? 'w-[280px]' : 'w-12',
      )}
    >
      <Button
        variant="ghost"
        size="sm"
        className="flex min-h-11 w-full items-center justify-between gap-2 px-2"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        <span className={cn('text-2xs font-semibold text-slate-900', !expanded && 'sr-only')}>
          {MAP_LABELS.legend}
        </span>
        {expanded ? (
          <ChevronLeft className="size-4 shrink-0" aria-hidden />
        ) : (
          <ChevronRight className="size-4 shrink-0" aria-hidden />
        )}
      </Button>
      {expanded && (
        <ul className="flex max-w-[280px] flex-col gap-1.5 px-3 pb-2 text-2xs text-slate-700">
          <li className="flex items-center gap-2">
            <span aria-hidden className="size-2.5 rounded-full bg-verdict-normal" />
            {VERDICT_LABEL.normal}
          </li>
          <li className="flex items-center gap-2">
            <span aria-hidden className="size-2.5 rounded-full bg-verdict-candidate" />
            {VERDICT_LABEL.candidate}
          </li>
          <li className="flex items-center gap-2">
            <span aria-hidden className="size-2.5 rounded-full bg-verdict-confirmed" />
            {VERDICT_LABEL.confirmed}
          </li>
          <li className="flex items-center gap-2">
            <span aria-hidden className="size-2.5 rounded-full bg-verdict-insufficient" />
            {VERDICT_LABEL.insufficient_data}
          </li>
          <li className="flex items-center gap-2">
            <span
              aria-hidden
              className="size-2.5 rounded-full border-2 border-dashed border-verdict-none"
            />
            {MAP_LABELS.legendNone}
          </li>
          <li className="flex items-center gap-2">
            <span
              aria-hidden
              className={cn('size-2.5 rounded-full border-2 border-dashed border-contour')}
            />
            {MAP_LABELS.legendContour}
          </li>
          <li className="flex items-center gap-2">
            <span
              aria-hidden
              className="size-2.5 rounded-full border-2 border-area-selected-outline"
            />
            {MAP_LABELS.legendSelected}
          </li>
        </ul>
      )}
    </div>
  );
}

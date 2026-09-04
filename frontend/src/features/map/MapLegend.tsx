import { Button } from '@/components/ui/button';
import { MAP_LABELS, VERDICT_LABEL } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { useState } from 'react';

/**
 * Легенда карты (план §6.4): статусы вывода, «не анализировался»,
 * «найденный контур», «выбранный». На мобиле сворачивается.
 */
export function MapLegend({ variant }: { variant: 'desktop' | 'mobile' }) {
  const [expanded, setExpanded] = useState(variant === 'desktop');

  return (
    <div className="absolute bottom-3 left-3 z-10 rounded-md border border-border bg-surface/95 shadow-1 backdrop-blur-sm">
      <Button
        variant="ghost"
        size="sm"
        className="flex w-full min-h-9 items-center justify-between gap-2 px-2"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        <span className="text-2xs font-medium">{MAP_LABELS.legend}</span>
        {expanded ? (
          <ChevronDown className="size-3.5" aria-hidden />
        ) : (
          <ChevronUp className="size-3.5" aria-hidden />
        )}
      </Button>
      {expanded && (
        <ul className="flex flex-col gap-1.5 px-3 pb-2 text-2xs text-ink-secondary">
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

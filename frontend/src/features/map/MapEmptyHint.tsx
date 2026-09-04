import { Button } from '@/components/ui/button';
import { EMPTY, SCAFFOLD } from '@/lib/labels';
import { Search } from 'lucide-react';

/**
 * Пустое состояние без участков (план §8): компактная карточка поверх карты
 * с текстом из брифа и двумя действиями основного пути (§3A).
 */
export function MapEmptyHint({ onSearch, onDraw }: { onSearch: () => void; onDraw: () => void }) {
  return (
    <div className="absolute left-1/2 top-1/2 z-10 w-[calc(100%-24px)] max-w-sm -translate-x-1/2 -translate-y-1/2 rounded-md border border-border bg-surface/95 p-4 text-center shadow-2 backdrop-blur-sm">
      <p className="mb-3 text-sm text-ink-secondary">{EMPTY.noAreas}</p>
      <div className="flex flex-wrap items-center justify-center gap-2">
        <Button size="sm" className="min-h-11" onClick={onSearch}>
          <Search aria-hidden />
          {SCAFFOLD.findContours}
        </Button>
        <Button variant="outline" size="sm" className="min-h-11" onClick={onDraw}>
          {SCAFFOLD.drawArea}
        </Button>
      </div>
    </div>
  );
}

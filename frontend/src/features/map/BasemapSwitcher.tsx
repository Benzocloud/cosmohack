import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { MAP_LABELS } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { useUi } from '@/store/ui';
import { Map as MapIcon, Satellite } from 'lucide-react';

/**
 * Переключатель подложки в углу карты (уточнение FE-3).
 * Подпись = режим, в который переключаемся; выбор живёт в ui-store + localStorage.
 */
export function BasemapSwitcher({ className }: { className?: string }) {
  const basemap = useUi((s) => s.basemap);
  const setBasemap = useUi((s) => s.setBasemap);
  const toSatellite = basemap === 'map';

  const label = toSatellite ? MAP_LABELS.basemapToSatellite : MAP_LABELS.basemapToMap;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className={cn('min-h-11 border-slate-300 bg-white text-slate-900 shadow-2', className)}
          aria-label={label}
          onClick={() => setBasemap(toSatellite ? 'satellite' : 'map')}
        >
          {toSatellite ? <Satellite aria-hidden /> : <MapIcon aria-hidden />}
          {label}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="left">
        {toSatellite ? 'Переключить на спутниковую подложку' : 'Переключить на обычную карту'}
      </TooltipContent>
    </Tooltip>
  );
}

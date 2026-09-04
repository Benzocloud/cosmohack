import { Button } from '@/components/ui/button';
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

  return (
    <Button
      variant="outline"
      size="sm"
      className={cn('min-h-11 bg-surface/95 shadow-1 backdrop-blur-sm', className)}
      aria-label={toSatellite ? MAP_LABELS.basemapToSatellite : MAP_LABELS.basemapToMap}
      onClick={() => setBasemap(toSatellite ? 'satellite' : 'map')}
    >
      {toSatellite ? <Satellite aria-hidden /> : <MapIcon aria-hidden />}
      {toSatellite ? MAP_LABELS.basemapToSatellite : MAP_LABELS.basemapToMap}
    </Button>
  );
}

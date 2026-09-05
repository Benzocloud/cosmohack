import type { Contour } from '@/api/adapters/contours';
import { Button } from '@/components/ui/button';
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from '@/components/ui/drawer';
import { MAP_LABELS, SCAFFOLD } from '@/lib/labels';
import { X } from 'lucide-react';

/**
 * Карточка найденного контура: источник, площадь, «Добавить участок».
 * Десктоп — карточка у точки клика (план §8), мобиль — Drawer снизу (§6.3).
 * Площадь считается в lib/geo (геометрия, не анализ).
 */
export interface ContourPopoverState {
  contour: Contour;
  areaHa: string;
  point: { x: number; y: number };
}

function ContourCardContent({
  contour,
  areaHa,
  onAdd,
}: {
  contour: Contour;
  areaHa: string;
  onAdd: () => void;
}) {
  return (
    <div className="flex flex-col gap-2">
      <p className="text-2xs text-ink-tertiary">{MAP_LABELS.contourSource}</p>
      <p className="text-sm font-semibold">{contour.name ?? contour.id}</p>
      <p className="text-2xs text-ink-secondary">{MAP_LABELS.areaHa(areaHa)}</p>
      <Button size="sm" className="min-h-11" onClick={onAdd}>
        {SCAFFOLD.addAreaLong}
      </Button>
    </div>
  );
}

export function ContourPopover({
  state,
  onClose,
  onAdd,
  variant,
}: {
  state: ContourPopoverState | null;
  onClose: () => void;
  onAdd: (contour: Contour) => void;
  variant: 'desktop' | 'mobile';
}) {
  if (!state) return null;
  const { contour, areaHa, point } = state;

  if (variant === 'mobile') {
    return (
      <Drawer open onOpenChange={(open) => !open && onClose()}>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle className="text-base">{contour.name ?? contour.id}</DrawerTitle>
          </DrawerHeader>
          <div className="px-4 pb-6">
            <ContourCardContent
              contour={contour}
              areaHa={areaHa}
              onAdd={() => {
                onClose();
                onAdd(contour);
              }}
            />
          </div>
        </DrawerContent>
      </Drawer>
    );
  }

  // Десктоп: карточка у точки клика; клампинг выполнен в MapView по контейнеру
  return (
    <aside
      aria-label={contour.name ?? contour.id}
      className="absolute z-20 w-64 rounded-md border border-border bg-surface p-3 shadow-2"
      style={{ left: point.x, top: point.y }}
    >
      <button
        type="button"
        aria-label="Закрыть"
        onClick={onClose}
        className="tap-target absolute right-1 top-1 flex items-center justify-center rounded-sm text-ink-tertiary hover:text-ink"
      >
        <X className="size-4" aria-hidden />
      </button>
      <ContourCardContent
        contour={contour}
        areaHa={areaHa}
        onAdd={() => {
          onClose();
          onAdd(contour);
        }}
      />
    </aside>
  );
}

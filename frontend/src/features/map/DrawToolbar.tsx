import { Button } from '@/components/ui/button';
import type { PolygonValidation } from '@/lib/geo';
import { MAP_LABELS } from '@/lib/labels';

/**
 * Панель свободного обведения: пользователь ведёт непрерывную линию и отпускает
 * палец/мышь для замыкания контура.
 */
export function DrawToolbar({
  validation,
  warning,
  onCancel,
}: {
  validation: PolygonValidation;
  warning?: string | null;
  onCancel: () => void;
}) {
  return (
    <div className="absolute bottom-4 left-1/2 z-20 flex w-[calc(100%-24px)] max-w-md -translate-x-1/2 flex-col gap-2 rounded-md border border-border bg-surface/95 p-3 shadow-2 backdrop-blur-sm">
      <div className="flex items-center justify-between gap-2 text-2xs text-ink-secondary">
        <span>{MAP_LABELS.drawHint}</span>
        <span className="whitespace-nowrap">{MAP_LABELS.releaseToClose}</span>
      </div>
      {!validation.ok && validation.error && (
        <p className="text-2xs text-verdict-confirmed">{validation.error}</p>
      )}
      {warning && (
        <p className="text-2xs text-verdict-confirmed" role="alert">
          {warning}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="ghost" size="sm" className="min-h-11" onClick={onCancel}>
          {MAP_LABELS.cancelDraw}
        </Button>
      </div>
    </div>
  );
}

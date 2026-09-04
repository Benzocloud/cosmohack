import { Button } from '@/components/ui/button';
import type { PolygonValidation } from '@/lib/geo';
import { MAP_LABELS } from '@/lib/labels';

/**
 * Панель рисования (план §6.4): подсказка, счётчик вершин, «Отменить точку» /
 * «Завершить» (активна при валидном черновике) / «Отмена». Видна только в режиме.
 * Ошибка валидации выводится рядом с «Завершить» (бриф §3A).
 */
export function DrawToolbar({
  vertexCount,
  validation,
  onUndo,
  onFinish,
  onCancel,
}: {
  vertexCount: number;
  validation: PolygonValidation;
  onUndo: () => void;
  onFinish: () => void;
  onCancel: () => void;
}) {
  return (
    <div className="absolute bottom-4 left-1/2 z-20 flex w-[calc(100%-24px)] max-w-md -translate-x-1/2 flex-col gap-2 rounded-md border border-border bg-surface/95 p-3 shadow-2 backdrop-blur-sm">
      <div className="flex items-center justify-between gap-2 text-2xs text-ink-secondary">
        <span>{MAP_LABELS.drawHint}</span>
        <span className="whitespace-nowrap">{MAP_LABELS.vertexCount(vertexCount)}</span>
      </div>
      {!validation.ok && validation.error && (
        <p className="text-2xs text-verdict-confirmed">{validation.error}</p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          className="min-h-11"
          disabled={vertexCount === 0}
          onClick={onUndo}
        >
          {MAP_LABELS.undoVertex}
        </Button>
        <Button size="sm" className="min-h-11" disabled={!validation.ok} onClick={onFinish}>
          {MAP_LABELS.finishDraw}
        </Button>
        <Button variant="ghost" size="sm" className="min-h-11" onClick={onCancel}>
          {MAP_LABELS.cancelDraw}
        </Button>
      </div>
    </div>
  );
}

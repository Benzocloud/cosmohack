import type { ContoursResult } from '@/api/adapters/contours';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { EMPTY, MAP_LABELS, SCAFFOLD } from '@/lib/labels';
import { Info, LoaderCircle, Search } from 'lucide-react';

/**
 * Кнопка поиска контуров со состояниями плана §6.4:
 * idle / loading / empty / failed / ok / stale. Запрос делает MapView по кнопке;
 * после сдвига карты состояние становится stale.
 */
export function ContoursButton({
  loading,
  stale,
  result,
  error,
  onSearch,
  onDraw,
}: {
  loading: boolean;
  stale: boolean;
  result: ContoursResult | null;
  error?: unknown;
  onSearch: () => void;
  onDraw: () => void;
}) {
  const status = error ? 'failed' : (result?.status ?? null);
  const okResult = result?.status === 'ok' ? result : null;
  const hasResult = status === 'ok' || status === 'empty' || status === 'failed';

  const label = loading
    ? MAP_LABELS.searching
    : stale && hasResult
      ? MAP_LABELS.stale
      : status === 'empty'
        ? EMPTY.contoursNotFound
        : status === 'failed'
          ? 'Повторить поиск контуров'
          : SCAFFOLD.findContours;

  return (
    <div className="flex max-w-[calc(100%-24px)] flex-col gap-1 rounded-md border border-border bg-surface/95 p-1.5 shadow-1 backdrop-blur-sm">
      <div className="flex items-start gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button size="sm" className="min-h-11 flex-1" disabled={loading} onClick={onSearch}>
              {loading ? (
                <LoaderCircle className="animate-spin" aria-hidden />
              ) : (
                <Search aria-hidden />
              )}
              {label}
            </Button>
          </TooltipTrigger>
          <TooltipContent>Найти сельхозконтуры в текущей области карты</TooltipContent>
        </Tooltip>
      </div>

      {/* ok: N контуров + примечание о неполноте каталога по ⓘ */}
      {okResult && !stale && (
        <div className="flex items-center justify-between gap-2 px-1 text-2xs text-ink-secondary">
          <span>{MAP_LABELS.contoursCount(okResult.contours.length)}</span>
          {okResult.coverageNote && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-6 min-h-6 min-w-6"
                  aria-label={okResult.coverageNote}
                >
                  <Info className="size-3.5" aria-hidden />
                </Button>
              </TooltipTrigger>
              <TooltipContent className="max-w-64">{okResult.coverageNote}</TooltipContent>
            </Tooltip>
          )}
        </div>
      )}

      {/* empty: не ошибка — предлагаем нарисовать свой участок */}
      {status === 'empty' && !stale && (
        <Button variant="outline" size="sm" className="min-h-11" onClick={onDraw}>
          {SCAFFOLD.drawArea}
        </Button>
      )}

      {/* failed: повтор запроса тем же действием */}
      {status === 'failed' && !stale && (
        <p className="px-1 text-2xs text-verdict-confirmed" role="alert">
          {EMPTY.contoursFailed}. Проверьте область и повторите запрос.
        </p>
      )}
    </div>
  );
}

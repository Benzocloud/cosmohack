import { type AppError, isAppError } from '@/api/client';
import { useCreateArea, useStartAnalysis } from '@/api/mutations';
import { useLimits } from '@/api/queries';
import type { Area, Period } from '@/api/types';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { polygonAreaHa, validatePeriod, validatePolygon } from '@/lib/geo';
import { DIALOG_LABELS, MAP_LABELS } from '@/lib/labels';
import { useEffect, useState } from 'react';

/**
 * Диалог добавления участка (план §6.4, бриф §3A).
 * Лимиты площади/периода/вершин — только из useLimits; числа не хардкодятся.
 * 422 показывается текстом сервера, диалог не закрывается (corrections §1).
 * «Сохранено, запуск не удался» → диалог закрывается, участок остаётся
 * с кнопкой «Запустить анализ» (ui.pendingStart).
 */

export interface AddAreaSource {
  kind: 'contour' | 'drawn';
  label: string;
  externalId?: string;
  provider?: string;
  /** Имя контура — автозаполнение названия. */
  name?: string;
  geometry: GeoJSON.Polygon;
}

const isoDate = (date: Date): string => date.toISOString().slice(0, 10);

export function AddAreaDialog({
  source,
  open,
  areasCount,
  onOpenChange,
  onSaved,
}: {
  source: AddAreaSource | null;
  open: boolean;
  areasCount: number;
  onOpenChange: (open: boolean) => void;
  onSaved: (area: Area, startError: AppError | null, period: Period) => void;
}) {
  const limits = useLimits().data ?? null;

  const [name, setName] = useState('');
  const [nameError, setNameError] = useState<string | null>(null);
  const [period, setPeriod] = useState<Period>({ from: '', to: '' });
  const [periodError, setPeriodError] = useState<string | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);
  const [cropType, setCropType] = useState('');

  const createMutation = useCreateArea();
  const startMutation = useStartAnalysis();

  // Черновик/контур подставляются при каждом открытии
  useEffect(() => {
    if (!open || !source) return;
    setName(source.name ?? `Участок ${areasCount + 1}`);
    setNameError(null);
    setPeriodError(null);
    setServerError(null);
    setCropType('');
    // Период по умолчанию — последние полгода до сегодня (UTC); лимиты periodDaysMax
    // проверяются в validatePeriod, числа лимитов из бэкенда
    const to = new Date();
    const from = new Date(to.getTime() - 183 * 86_400_000);
    setPeriod({ from: isoDate(from), to: isoDate(to) });
  }, [open, source, areasCount]);

  const pending = createMutation.isPending || startMutation.isPending;
  const areaHa = source ? polygonAreaHa(toRing(source.geometry)) : 0;
  const geometryCheck = source ? validatePolygon(toRing(source.geometry), limits) : null;

  async function submit(andAnalyze: boolean) {
    if (!source) return;
    setServerError(null);

    if (geometryCheck && !geometryCheck.ok) {
      setServerError(geometryCheck.error ?? 'Полигон невалиден');
      return;
    }

    if (!name.trim()) {
      setNameError(DIALOG_LABELS.nameRequired);
      return;
    }
    setNameError(null);

    const periodCheck = validatePeriod(period.from, period.to, limits);
    if (!periodCheck.ok) {
      setPeriodError(periodCheck.error ?? null);
      return;
    }
    setPeriodError(null);

    try {
      const area = await createMutation.mutateAsync({
        name: name.trim(),
        geometry: source.geometry,
        sourceKind: source.kind,
        sourceLabel: source.label,
        sourceExternalId: source.externalId,
        sourceProvider: source.provider,
        cropType,
        period,
      });
      if (!andAnalyze) {
        onSaved(area, null, period);
        return;
      }
      // 429/ошибки запуска не отменяют сохранение: участок остаётся в списке
      try {
        await startMutation.mutateAsync({ areaId: area.id, period });
        onSaved(area, null, period);
      } catch (startError) {
        onSaved(area, isAppError(startError) ? startError : null, period);
      }
    } catch (createError) {
      // 422/400: текст сервера у формы, диалог открыт (corrections §1)
      if (isAppError(createError)) {
        setServerError(createError.detail ?? createError.title);
      } else {
        setServerError(String(createError));
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {source && (
        <DialogContent aria-label={DIALOG_LABELS.addAndAnalyze}>
          <DialogHeader>
            <DialogTitle className="text-base">{DIALOG_LABELS.addAndAnalyze}</DialogTitle>
            <DialogDescription className="text-2xs">
              {source.label}
              {source.externalId ? ` · ${source.externalId}` : ''}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="area-name">{DIALOG_LABELS.name}</Label>
              <Input
                id="area-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                aria-invalid={!!nameError}
                autoComplete="off"
              />
              {nameError && <p className="text-2xs text-verdict-confirmed">{nameError}</p>}
            </div>

            <div className="grid gap-1.5">
              <Label>
                {DIALOG_LABELS.period} ({DIALOG_LABELS.from} / {DIALOG_LABELS.to})
              </Label>
              <div className="flex items-center gap-2">
                <Input
                  type="date"
                  value={period.from}
                  min={limits?.minDate}
                  onChange={(event) => setPeriod((p) => ({ ...p, from: event.target.value }))}
                  aria-label={`${DIALOG_LABELS.period} ${DIALOG_LABELS.from}`}
                />
                <Input
                  type="date"
                  value={period.to}
                  min={limits?.minDate}
                  onChange={(event) => setPeriod((p) => ({ ...p, to: event.target.value }))}
                  aria-label={`${DIALOG_LABELS.period} ${DIALOG_LABELS.to}`}
                />
              </div>
              {periodError && <p className="text-2xs text-verdict-confirmed">{periodError}</p>}
            </div>

            <div className="grid grid-cols-2 gap-3 text-2xs text-ink-secondary">
              <div>
                <p className="font-medium text-ink">{DIALOG_LABELS.source}</p>
                <p>{source.label}</p>
              </div>
              <div>
                <p className="font-medium text-ink">{DIALOG_LABELS.area}</p>
                <p>{MAP_LABELS.areaHa(areaHa.toFixed(1))}</p>
              </div>
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="crop-type">Культура (необязательно)</Label>
              <Input
                id="crop-type"
                value={cropType}
                onChange={(event) => setCropType(event.target.value)}
                placeholder="например, пшеница"
                autoComplete="off"
              />
            </div>

            {serverError && (
              <p className="text-2xs text-verdict-confirmed" role="alert">
                {serverError}
              </p>
            )}
          </div>

          <DialogFooter className="gap-2">
            <Button
              size="sm"
              className="min-h-11"
              disabled={pending}
              onClick={() => void submit(true)}
            >
              {DIALOG_LABELS.addAndAnalyze}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="min-h-11"
              disabled={pending}
              onClick={() => void submit(false)}
            >
              {DIALOG_LABELS.saveOnly}
            </Button>
          </DialogFooter>
        </DialogContent>
      )}
    </Dialog>
  );
}

/** MultiPolygon в диалог не попадает (рисуем только полигон), но адаптируем на всякий случай. */
function toRing(geometry: GeoJSON.Polygon | GeoJSON.MultiPolygon): [number, number][] {
  return (geometry.type === 'Polygon' ? geometry.coordinates[0] : geometry.coordinates[0][0]) as [
    number,
    number,
  ][];
}

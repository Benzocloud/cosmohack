import { useArea, useAreas, useResultBundle } from '@/api/queries';
import { Button } from '@/components/ui/button';
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from '@/components/ui/drawer';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { AnalysisTimeline, AreaCard, type ResultViewProps } from '@/features/analysis/ResultPanel';
import { AreaList } from '@/features/areas/AreaList';
import { MapView } from '@/features/map/MapView';
import { Header } from '@/features/shell/Header';
import { MobileTabBar } from '@/features/shell/MobileTabBar';
import { EMPTY, SCAFFOLD } from '@/lib/labels';
import { useBreakpoint } from '@/lib/useMediaQuery';
import { useSelection } from '@/store/selection';
import { useUi } from '@/store/ui';
import { PanelRight, Rows3 } from 'lucide-react';
import { useState } from 'react';

/**
 * Каркас приложения по §6.1–6.3.
 * Десктоп ≥1280: список 300 | карта | карточка 380, низ 440px.
 * Планшет 1024–1279: рейка 56px + Sheet слева, карточка — Sheet справа, низ 400px.
 * Мобильный <1024: вкладки Участки/Карта/Анализ, шапка 48px.
 *
 * FE-3: центральная зона — MapView (карта, контуры, рисование), слева —
 * минимальный список участков. Карточка и график остаются заглушками (FE-2/FE-4).
 */

export function AreasPanel() {
  const areasQuery = useAreas();
  const areasCount = areasQuery.data?.length ?? 0;
  const requestDraw = useUi((s) => s.requestDraw);
  const setMobileTab = useUi((s) => s.setMobileTab);
  return (
    <section aria-label={SCAFFOLD.areasPanel} className="flex h-full flex-col overflow-y-auto">
      <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
        <h2 className="text-sm font-semibold">{SCAFFOLD.areasPanel}</h2>
        <Button
          size="sm"
          variant="outline"
          className="min-h-11"
          onClick={() => {
            setMobileTab('map');
            requestDraw();
          }}
          aria-label={SCAFFOLD.drawArea}
        >
          {SCAFFOLD.drawArea}
        </Button>
      </div>
      {areasQuery.isPending ? (
        <div className="space-y-2 p-4" aria-label="Загрузка списка участков">
          <div className="h-10 animate-pulse rounded-md bg-surface-muted" />
          <div className="h-10 animate-pulse rounded-md bg-surface-muted" />
          <div className="h-10 animate-pulse rounded-md bg-surface-muted" />
        </div>
      ) : areasQuery.isError ? (
        <div className="space-y-3 p-4" role="alert">
          <p className="text-sm text-verdict-confirmed">Не удалось загрузить список участков.</p>
          <Button size="sm" variant="outline" onClick={() => void areasQuery.refetch()}>
            Повторить
          </Button>
        </div>
      ) : areasCount > 0 ? (
        <AreaList />
      ) : (
        <p className="p-4 text-sm text-ink-secondary">{EMPTY.noAreas}</p>
      )}
    </section>
  );
}

function useSelectedResult(): ResultViewProps {
  const selectedAreaId = useSelection((s) => s.selectedAreaId);
  const areasQuery = useAreas();
  const areaQuery = useArea(selectedAreaId);
  // Список даёт имя/геометрию сразу, detail-запрос добавляет актуальный результат.
  const listArea = areasQuery.data?.find((area) => area.id === selectedAreaId);
  const area = areaQuery.data ?? listArea;
  const result = useResultBundle(selectedAreaId, area?.lastResult?.resultVersion ?? null);

  return {
    area,
    areaLoading: areaQuery.isPending && !area,
    areaError: areaQuery.error,
    result,
    onRetryArea: () => void areaQuery.refetch(),
    onRetryResult: () => void result.refetch(),
  };
}

function DesktopLayout() {
  const result = useSelectedResult();
  return (
    <div className="flex min-h-0 flex-1">
      <aside className="w-[300px] shrink-0 border-r border-border bg-surface">
        <AreasPanel />
      </aside>
      <main className="flex min-w-0 flex-1 flex-col">
        <div className="min-h-0 flex-1">
          <MapView variant="desktop" />
        </div>
        {/* Нижняя зона 440px: график ~300 + погода (§2.4) */}
        <div className="h-[440px] shrink-0 border-t border-border bg-surface">
          <AnalysisTimeline {...result} />
        </div>
      </main>
      <aside className="w-[380px] shrink-0 border-l border-border bg-surface">
        <AreaCard {...result} />
      </aside>
    </div>
  );
}

function TabletLayout() {
  const result = useSelectedResult();
  const [listOpen, setListOpen] = useState(false);
  const cardOpen = useUi((s) => s.cardOpen);
  const setCardOpen = useUi((s) => s.setCardOpen);

  return (
    <div className="flex min-h-0 flex-1">
      {/* Рейка 56px: список участков уезжает в Sheet (§6.2) */}
      <aside className="flex w-14 shrink-0 flex-col items-center border-r border-border bg-surface py-2">
        <Sheet open={listOpen} onOpenChange={setListOpen}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={SCAFFOLD.openAreasList}>
              <Rows3 aria-hidden />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-[300px] p-0">
            <SheetHeader className="border-b border-border">
              <SheetTitle className="text-sm">{SCAFFOLD.areasPanel}</SheetTitle>
            </SheetHeader>
            <div className="h-[calc(100%-64px)] overflow-y-auto">
              <AreasPanel />
            </div>
          </SheetContent>
        </Sheet>
      </aside>
      <main className="flex min-w-0 flex-1 flex-col">
        <div className="relative min-h-0 flex-1">
          <MapView variant="desktop" />
          {/* Карточка — выезжающая панель поверх карты (§6.2) */}
          <Sheet open={cardOpen} onOpenChange={setCardOpen}>
            <SheetTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                aria-label={SCAFFOLD.openAreaCard}
                className="absolute right-3 top-3 z-10 shadow-1"
              >
                <PanelRight aria-hidden />
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-[380px] p-0 sm:max-w-[380px]">
              <SheetHeader className="border-b border-border">
                <SheetTitle className="text-sm">{SCAFFOLD.openAreaCard}</SheetTitle>
              </SheetHeader>
              <AreaCard {...result} />
            </SheetContent>
          </Sheet>
        </div>
        {/* Низ 400px на планшете (§6.2) */}
        <div className="h-[400px] shrink-0 border-t border-border bg-surface">
          <AnalysisTimeline {...result} />
        </div>
      </main>
    </div>
  );
}

function MobileLayout() {
  const result = useSelectedResult();
  const mobileTab = useUi((s) => s.mobileTab);
  const cardOpen = useUi((s) => s.cardOpen);
  const setCardOpen = useUi((s) => s.setCardOpen);

  return (
    <>
      <main className="min-h-0 flex-1">
        {mobileTab === 'areas' && <AreasPanel />}
        {mobileTab === 'map' && (
          <div className="relative h-full">
            <MapView variant="mobile" />
            {/* Краткая карточка участка — выезжает снизу (§6.3) */}
            <Drawer open={cardOpen} onOpenChange={setCardOpen}>
              <DrawerContent className="max-h-[70vh]">
                <DrawerHeader>
                  <DrawerTitle className="text-base">{SCAFFOLD.openAreaCard}</DrawerTitle>
                </DrawerHeader>
                <AreaCard {...result} />
              </DrawerContent>
            </Drawer>
          </div>
        )}
        {mobileTab === 'analysis' && <AnalysisTimeline {...result} />}
      </main>
      <MobileTabBar />
    </>
  );
}

export function AppShell() {
  const breakpoint = useBreakpoint();

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-app text-[13px]">
      <Header />
      {breakpoint === 'desktop' && <DesktopLayout />}
      {breakpoint === 'tablet' && <TabletLayout />}
      {breakpoint === 'mobile' && <MobileLayout />}
    </div>
  );
}

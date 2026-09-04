import { Button } from '@/components/ui/button';
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from '@/components/ui/drawer';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Header } from '@/features/shell/Header';
import { MobileTabBar } from '@/features/shell/MobileTabBar';
import { EMPTY, SCAFFOLD } from '@/lib/labels';
import { useBreakpoint } from '@/lib/useMediaQuery';
import { useUi } from '@/store/ui';
import { PanelRight, PenLine, Rows3, Search } from 'lucide-react';
import { useState } from 'react';

/**
 * Каркас приложения по §6.1–6.3.
 * Десктоп ≥1280: список 300 | карта | карточка 380, низ 440px.
 * Планшет 1024–1279: рейка 56px + Sheet слева, карточка — Sheet справа, низ 400px.
 * Мобильный <1024: вкладки Участки/Карта/Анализ, шапка 48px.
 *
 * SCAFFOLD: зоны-заглушки внутри панелей заменяются фичами на FE-2 (список/карточка),
 * FE-3 (карта) и FE-4 (график/погода) — тексты берутся из labels (SCAFFOLD/EMPTY).
 */

function AreasPanel() {
  return (
    <section aria-label={SCAFFOLD.areasPanel} className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
        <h2 className="text-sm font-semibold">{SCAFFOLD.areasPanel}</h2>
        {/* AddAreaDialog — FE-3 */}
        <Button size="sm" disabled>
          {SCAFFOLD.addArea}
        </Button>
      </div>
      {/* Честное пустое состояние: текст из брифа, а не демо-данные */}
      <p className="p-4 text-sm text-ink-secondary">{EMPTY.noAreas}</p>
    </section>
  );
}

function MapPlaceholder() {
  return (
    <section
      aria-label={SCAFFOLD.mapPanel}
      className="flex h-full min-h-0 flex-col items-center justify-center gap-3 bg-surface-muted p-4"
    >
      <p className="text-sm text-ink-tertiary">{SCAFFOLD.placeholderMap}</p>
      {/* Заглушки основного пути добавления (бриф §3A); MapLibre и поиск контуров — FE-3 */}
      <div className="flex flex-wrap items-center justify-center gap-2">
        <Button variant="outline" size="sm" disabled>
          <Search aria-hidden />
          {SCAFFOLD.findContours}
        </Button>
        <Button variant="outline" size="sm" disabled>
          <PenLine aria-hidden />
          {SCAFFOLD.drawArea}
        </Button>
      </div>
    </section>
  );
}

function ChartPlaceholder() {
  return (
    <div
      aria-label={SCAFFOLD.placeholderChart}
      className="flex min-h-0 flex-1 items-center justify-center text-sm text-ink-tertiary"
    >
      {SCAFFOLD.placeholderChart}
    </div>
  );
}

function WeatherPlaceholder() {
  return (
    <div
      aria-label={SCAFFOLD.placeholderWeather}
      className="flex items-center justify-center border-t border-border text-sm text-ink-tertiary"
    >
      {SCAFFOLD.placeholderWeather}
    </div>
  );
}

function CardPanel() {
  return (
    <section aria-label={SCAFFOLD.openAreaCard} className="flex h-full flex-col">
      <div className="border-b border-border px-4 py-3">
        <h2 className="text-sm font-semibold">{SCAFFOLD.noAreaSelected}</h2>
      </div>
      <div className="flex flex-1 items-center justify-center p-6 text-center text-sm text-ink-tertiary">
        {SCAFFOLD.placeholderCard}
      </div>
    </section>
  );
}

function DesktopLayout() {
  return (
    <div className="flex min-h-0 flex-1">
      <aside className="w-[300px] shrink-0 border-r border-border bg-surface">
        <AreasPanel />
      </aside>
      <main className="flex min-w-0 flex-1 flex-col">
        <div className="min-h-0 flex-1">
          <MapPlaceholder />
        </div>
        {/* Нижняя зона 440px: график ~300 + погода (§2.4) */}
        <div className="grid h-[440px] shrink-0 grid-rows-[1fr_140px] border-t border-border bg-surface">
          <ChartPlaceholder />
          <WeatherPlaceholder />
        </div>
      </main>
      <aside className="w-[380px] shrink-0 border-l border-border bg-surface">
        <CardPanel />
      </aside>
    </div>
  );
}

function TabletLayout() {
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
            <p className="p-4 text-sm text-ink-secondary">{EMPTY.noAreas}</p>
          </SheetContent>
        </Sheet>
      </aside>
      <main className="flex min-w-0 flex-1 flex-col">
        <div className="relative min-h-0 flex-1">
          <MapPlaceholder />
          {/* Карточка — выезжающая панель поверх карты (§6.2) */}
          <Sheet open={cardOpen} onOpenChange={setCardOpen}>
            <SheetTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                aria-label={SCAFFOLD.openAreaCard}
                className="absolute right-3 top-3 shadow-1"
              >
                <PanelRight aria-hidden />
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-[380px] p-0 sm:max-w-[380px]">
              <SheetHeader className="border-b border-border">
                <SheetTitle className="text-sm">{SCAFFOLD.openAreaCard}</SheetTitle>
              </SheetHeader>
              <p className="p-4 text-sm text-ink-secondary">{SCAFFOLD.placeholderCard}</p>
            </SheetContent>
          </Sheet>
        </div>
        {/* Низ 400px на планшете (§6.2) */}
        <div className="grid h-[400px] shrink-0 grid-rows-[1fr_120px] border-t border-border bg-surface">
          <ChartPlaceholder />
          <WeatherPlaceholder />
        </div>
      </main>
    </div>
  );
}

function MobileLayout() {
  const mobileTab = useUi((s) => s.mobileTab);
  const cardOpen = useUi((s) => s.cardOpen);
  const setCardOpen = useUi((s) => s.setCardOpen);

  return (
    <>
      <main className="min-h-0 flex-1">
        {mobileTab === 'areas' && <AreasPanel />}
        {mobileTab === 'map' && (
          <div className="relative h-full">
            <MapPlaceholder />
            {/* Краткая карточка участка — выезжает снизу (§6.3) */}
            <Drawer open={cardOpen} onOpenChange={setCardOpen}>
              <DrawerTrigger asChild>
                <Button
                  variant="outline"
                  size="icon"
                  aria-label={SCAFFOLD.openAreaCard}
                  className="absolute right-3 top-3 shadow-1"
                >
                  <PanelRight aria-hidden />
                </Button>
              </DrawerTrigger>
              <DrawerContent className="max-h-[70vh]">
                <DrawerHeader>
                  <DrawerTitle className="text-base">{SCAFFOLD.openAreaCard}</DrawerTitle>
                </DrawerHeader>
                <p className="px-4 pb-6 text-sm text-ink-secondary">{SCAFFOLD.placeholderCard}</p>
              </DrawerContent>
            </Drawer>
          </div>
        )}
        {mobileTab === 'analysis' && (
          <div className="flex h-full flex-col">
            {/* График 240px по ширине экрана (§6.3) */}
            <div className="flex h-[240px] shrink-0 items-center justify-center border-b border-border text-sm text-ink-tertiary">
              {SCAFFOLD.placeholderChart}
            </div>
            <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-ink-tertiary">
              {SCAFFOLD.placeholderWeather}
            </div>
          </div>
        )}
      </main>
      <MobileTabBar />
    </>
  );
}

export function AppShell() {
  const breakpoint = useBreakpoint();

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-app">
      <Header />
      {breakpoint === 'desktop' && <DesktopLayout />}
      {breakpoint === 'tablet' && <TabletLayout />}
      {breakpoint === 'mobile' && <MobileLayout />}
    </div>
  );
}

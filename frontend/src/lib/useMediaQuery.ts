import { useEffect, useState } from 'react';

/**
 * Реактивный matchMedia. На сервере/в jsdom без matchMedia возвращает false, не падая —
 * каркас должен рендериться в тестах и без браузера.
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState<boolean>(
    () => typeof window !== 'undefined' && window.matchMedia?.(query).matches === true,
  );

  useEffect(() => {
    const mql = window.matchMedia?.(query);
    if (!mql) return undefined;
    const onChange = (event: MediaQueryListEvent) => setMatches(event.matches);
    setMatches(mql.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, [query]);

  return matches;
}

/** Брейкпоинты компоновки §2.4: ≥1280 десктоп, 1024–1279 планшет, <1024 мобильный. */
export function useBreakpoint(): 'desktop' | 'tablet' | 'mobile' {
  const isDesktop = useMediaQuery('(min-width: 1280px)');
  const isTablet = useMediaQuery('(min-width: 1024px)');
  if (isDesktop) return 'desktop';
  if (isTablet) return 'tablet';
  return 'mobile';
}

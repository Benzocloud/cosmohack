import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { ReactNode } from 'react';

/** Провайдеры приложения. TanStack Query подключается на FE-1 (API-слой). */
export function Providers({ children }: { children: ReactNode }) {
  return (
    <TooltipProvider delayDuration={200}>
      {children}
      <Toaster position="top-center" />
    </TooltipProvider>
  );
}

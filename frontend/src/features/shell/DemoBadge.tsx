import { Badge } from '@/components/ui/badge';
import { EMPTY } from '@/lib/labels';
import { FlaskConical } from 'lucide-react';

/**
 * Бейдж «Демонстрационные данные» — обязателен в mock-режиме
 * (design-brief §8, corrections §5); скрывается вне моков.
 */
export function DemoBadge({ demo }: { demo: boolean }) {
  if (!demo) return null;
  return (
    <Badge
      variant="outline"
      data-testid="demo-badge"
      className="border-border-strong text-ink-secondary"
    >
      <FlaskConical aria-hidden />
      {EMPTY.demo}
    </Badge>
  );
}

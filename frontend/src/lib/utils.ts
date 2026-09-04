import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Каноничная утилита shadcn/ui для склейки классов с дедупликацией tailwind
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

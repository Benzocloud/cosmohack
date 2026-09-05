import { Toaster as Sonner, type ToasterProps } from 'sonner';

// Обёртка sonner для Vite (без next-themes: тема светлая — единственная в P0)
const Toaster = ({ ...props }: ToasterProps) => (
  <Sonner
    theme="light"
    className="toaster group"
    toastOptions={{
      classNames: {
        toast:
          'group toast group-[.toaster]:bg-background group-[.toaster]:text-ink group-[.toaster]:border-border group-[.toaster]:shadow-2',
        description: 'group-[.toast]:text-ink-secondary',
        actionButton: 'group-[.toast]:bg-primary group-[.toast]:text-primary-foreground',
        cancelButton: 'group-[.toast]:bg-muted group-[.toast]:text-muted-foreground',
      },
    }}
    {...props}
  />
);

export { Toaster };

import type { Config } from 'tailwindcss';
import animate from 'tailwindcss-animate';

/**
 * Дизайн-система AgroPulse (frontend-plan §2).
 * Все цвета — ссылки на CSS-переменные из src/styles/tokens.css (§2.2 дословно),
 * значения не дублируются: источник истины один — токены.
 * Плюс канонические алиасы shadcn/ui (background/primary/...), маппящиеся на те же токены.
 */
export default {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      // Шкала типографики §2.3: 12/13/14/16/20/24
      fontSize: {
        '2xs': ['12px', { lineHeight: '1.45' }],
        sm: ['13px', { lineHeight: '1.45' }],
        base: ['14px', { lineHeight: '1.45' }],
        lg: ['16px', { lineHeight: '1.25' }],
        xl: ['20px', { lineHeight: '1.25' }],
        '2xl': ['24px', { lineHeight: '1.25' }],
      },
      colors: {
        // --- алиасы shadcn/ui поверх токенов ---
        border: 'var(--border)',
        'border-strong': 'var(--border-strong)',
        input: 'var(--border)',
        ring: 'var(--focus-ring)',
        background: 'var(--bg-surface)',
        foreground: 'var(--fg)',
        primary: {
          DEFAULT: 'var(--action)',
          foreground: 'var(--fg-on-accent)',
        },
        secondary: {
          DEFAULT: 'var(--bg-muted)',
          foreground: 'var(--fg-secondary)',
        },
        muted: {
          DEFAULT: 'var(--bg-muted)',
          foreground: 'var(--fg-tertiary)',
        },
        accent: {
          DEFAULT: 'var(--bg-hover)',
          foreground: 'var(--fg)',
        },
        destructive: {
          DEFAULT: 'var(--verdict-confirmed)',
          foreground: 'var(--fg-on-accent)',
        },
        card: {
          DEFAULT: 'var(--bg-surface)',
          foreground: 'var(--fg)',
        },
        popover: {
          DEFAULT: 'var(--bg-surface)',
          foreground: 'var(--fg)',
        },
        // --- семантика проекта (§2.2) ---
        app: 'var(--bg-app)',
        surface: 'var(--bg-surface)',
        'surface-muted': 'var(--bg-muted)',
        'surface-hover': 'var(--bg-hover)',
        ink: {
          DEFAULT: 'var(--fg)',
          secondary: 'var(--fg-secondary)',
          tertiary: 'var(--fg-tertiary)',
          'on-accent': 'var(--fg-on-accent)',
        },
        action: {
          DEFAULT: 'var(--action)',
          hover: 'var(--action-hover)',
          soft: 'var(--action-soft)',
        },
        observed: 'var(--observed)',
        imputed: 'var(--imputed)',
        missing: 'var(--missing)',
        'background-band': 'var(--background-band)',
        'background-mean': 'var(--background-mean)',
        verdict: {
          normal: 'var(--verdict-normal)',
          candidate: 'var(--verdict-candidate)',
          confirmed: 'var(--verdict-confirmed)',
          insufficient: 'var(--verdict-insufficient)',
          none: 'var(--verdict-none)',
        },
        job: {
          queued: 'var(--job-queued)',
          running: 'var(--job-running)',
          failed: 'var(--job-failed)',
          cancelled: 'var(--job-cancelled)',
        },
        temp: 'var(--temp)',
        precip: 'var(--precip)',
        contour: 'var(--contour-found)',
        'area-selected-outline': 'var(--area-selected-outline)',
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        md: 'var(--radius)',
        lg: 'var(--radius-lg)',
      },
      boxShadow: {
        1: 'var(--shadow-1)',
        2: 'var(--shadow-2)',
      },
    },
  },
  plugins: [animate],
} satisfies Config;

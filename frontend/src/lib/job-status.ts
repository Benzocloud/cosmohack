import type { JobMeta } from '@/api/types';
import { JOB_LABEL, JOB_STAGE_FALLBACK, STAGE_LABEL } from '@/lib/labels';

/**
 * Подпись статуса задачи: только то, что реально передал backend (бриф §3B).
 * Стадия без ключа из STAGE_LABEL или без стадии → «Анализ выполняется».
 * Общая для dev-страницы (FE-1) и списка участков (FE-3).
 */
export function jobStatusText(job: JobMeta | undefined): { text: string; className: string } {
  if (!job) return { text: '—', className: 'text-ink-tertiary' };
  if (job.status === 'running') {
    const stage = job.stage
      ? (STAGE_LABEL[job.stage as keyof typeof STAGE_LABEL] ?? JOB_STAGE_FALLBACK)
      : JOB_STAGE_FALLBACK;
    return { text: `${JOB_LABEL.running} · ${stage}`, className: 'text-job-running' };
  }
  if (job.status === 'queued') return { text: JOB_LABEL.queued, className: 'text-job-queued' };
  if (job.status === 'failed') return { text: JOB_LABEL.failed, className: 'text-job-failed' };
  if (job.status === 'cancelled')
    return { text: JOB_LABEL.cancelled, className: 'text-job-cancelled' };
  return { text: JOB_LABEL.completed, className: 'text-ink' };
}

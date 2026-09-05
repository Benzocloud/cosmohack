import type { JobMeta, JobStatus } from '@/api/types';

/**
 * Адаптер задачи: GET /api/jobs/{id} (frontend-plan §4).
 * Статус задачи — самостоятельное поле, не смешивается с выводом анализа.
 */

export interface JobRaw {
  [key: string]: unknown;
  id?: string;
  area_id?: string;
  period?: { from?: string; to?: string };
  requested_period?: { from?: string; to?: string };
  status?: string;
  stage?: string | null;
  message?: string | null;
  result_version?: string | null;
  updated_at?: string;
  error?: { code?: string; message?: string; retryable?: boolean } | null;
}

export interface ActiveJobRaw {
  job_id?: string;
  status?: string;
  stage?: string | null;
}

const JOB_STATUSES: JobStatus[] = ['queued', 'running', 'completed', 'failed', 'cancelled'];

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptJob: required field ${field} is missing or empty`);
  }
  return value;
}

export function adaptJob(raw: JobRaw, areaIdFallback = ''): JobMeta {
  const status = JOB_STATUSES.find((s) => s === raw.status);
  if (!status) {
    throw new Error(`adaptJob: unknown job status ${String(raw.status)}`);
  }

  return {
    id: requireString(raw.id, 'id'),
    areaId: requireString(raw.area_id ?? areaIdFallback, 'area_id'),
    requestedPeriod: {
      from: requireString(raw.period?.from, 'period.from'),
      to: requireString(raw.period?.to, 'period.to'),
    },
    status,
    stage: raw.stage ?? null,
    message: raw.message ?? null,
    errorCode: raw.error?.code ?? null,
    errorMessage: raw.error?.message ?? null,
    resultVersion: raw.result_version ?? null,
    updatedAt: raw.updated_at ?? new Date(0).toISOString(),
  };
}

export function adaptActiveJob(
  raw: ActiveJobRaw,
  areaId: string,
  period: { from?: string; to?: string },
): JobMeta {
  return adaptJob(
    { id: raw.job_id, area_id: areaId, period, status: raw.status, stage: raw.stage },
    areaId,
  );
}

import type { JobMeta, JobStatus } from '@/api/types';

/**
 * Адаптер задачи: GET /api/jobs/{id} (frontend-plan §4).
 * Статус задачи — самостоятельное поле, не смешивается с выводом анализа.
 */

export interface JobRaw {
  id?: string;
  area_id?: string;
  requested_period?: { from?: string; to?: string };
  status?: string;
  stage?: string | null;
  message?: string | null;
  error_code?: string | null;
  error_message?: string | null;
  result_version?: string | null;
  updated_at?: string;
}

const JOB_STATUSES: JobStatus[] = ['queued', 'running', 'completed', 'failed', 'cancelled'];

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`adaptJob: обязательное поле ${field} отсутствует или пустое`);
  }
  return value;
}

export function adaptJob(raw: JobRaw): JobMeta {
  const status = JOB_STATUSES.find((s) => s === raw.status);
  if (!status) {
    throw new Error(`adaptJob: неизвестный статус задачи «${String(raw.status)}»`);
  }

  return {
    id: requireString(raw.id, 'id'),
    areaId: requireString(raw.area_id, 'area_id'),
    requestedPeriod: {
      from: requireString(raw.requested_period?.from, 'requested_period.from'),
      to: requireString(raw.requested_period?.to, 'requested_period.to'),
    },
    status,
    stage: raw.stage ?? null,
    message: raw.message ?? null,
    errorCode: raw.error_code ?? null,
    errorMessage: raw.error_message ?? null,
    resultVersion: raw.result_version ?? null,
    updatedAt: requireString(raw.updated_at, 'updated_at'),
  };
}

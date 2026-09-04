import {
  EMPTY,
  JOB_LABEL,
  PROVENANCE_LABEL,
  SCAFFOLD,
  STAGE_LABEL,
  VERDICT_LABEL,
} from '@/lib/labels';
import { describe, expect, it } from 'vitest';

/**
 * Полнота словаря строк: три оси состояний из брифа §5 должны иметь
 * все значения, и ни одна строка не должна быть пустой.
 */
describe('словарь строк интерфейса', () => {
  it('содержит все пять статусов задачи', () => {
    expect(Object.keys(JOB_LABEL).sort()).toEqual([
      'cancelled',
      'completed',
      'failed',
      'queued',
      'running',
    ]);
  });

  it('содержит все четыре статуса вывода', () => {
    expect(Object.keys(VERDICT_LABEL).sort()).toEqual([
      'candidate',
      'confirmed',
      'insufficient_data',
      'normal',
    ]);
  });

  it('содержит три типа происхождения значений', () => {
    expect(Object.keys(PROVENANCE_LABEL).sort()).toEqual(['imputed', 'missing', 'observed']);
  });

  it('не содержит пустых строк', () => {
    for (const dict of [JOB_LABEL, VERDICT_LABEL, PROVENANCE_LABEL, STAGE_LABEL, EMPTY, SCAFFOLD]) {
      for (const value of Object.values(dict)) {
        expect(value.length).toBeGreaterThan(0);
      }
    }
  });
});

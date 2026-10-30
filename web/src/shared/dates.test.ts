import { describe, expect, it } from 'vitest';
import { monthsBefore, today, validPeriod } from './dates';

describe('calendar periods', () => {
  it('clamps month ends and leap days', () => {
    expect(monthsBefore('2026-03-31', 1)).toBe('2026-02-28');
    expect(monthsBefore('2024-03-31', 1)).toBe('2024-02-29');
    expect(monthsBefore('2024-02-29', 12)).toBe('2023-02-28');
  });
  it('uses the date in São Paulo rather than UTC', () => {
    expect(today(new Date('2026-10-30T01:00:00Z'))).toBe('2026-10-29');
  });
  it('limits requests to the API window while supporting a leap year', () => {
    expect(validPeriod({ start: '2023-03-01', end: '2024-03-01' })).toBe(true);
    expect(validPeriod({ start: '2023-03-01', end: '2024-03-02' })).toBe(false);
  });
  it('rejects impossible, missing and reversed dates', () => {
    expect(validPeriod({ start: '2026-02-30', end: '2026-03-02' })).toBe(false);
    expect(validPeriod({ start: '', end: '2026-03-02' })).toBe(false);
    expect(validPeriod({ start: '2026-03-03', end: '2026-03-02' })).toBe(false);
    expect(validPeriod({ start: '2026-03-02', end: '2026-03-02' })).toBe(true);
  });
});

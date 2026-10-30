export const timeZone = 'America/Sao_Paulo';
export const presets = [1, 3, 6, 12] as const;
export type Period = { start: string; end: string };

export function today(now = new Date()): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone }).format(now);
}

export function monthsBefore(end: string, months: number): string {
  const date = new Date(`${end}T12:00:00Z`);
  const day = date.getUTCDate();
  date.setUTCDate(1);
  date.setUTCMonth(date.getUTCMonth() - months);
  const last = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 0));
  date.setUTCDate(Math.min(day, last.getUTCDate()));
  return date.toISOString().slice(0, 10);
}

export function validDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const date = new Date(`${value}T12:00:00Z`);
  return !Number.isNaN(date.getTime()) && date.toISOString().startsWith(value);
}

export function validPeriod(period: Period): boolean {
  return validDate(period.start) && validDate(period.end) && period.start <= period.end
    && Date.parse(period.end) - Date.parse(period.start) <= 366 * 24 * 60 * 60 * 1000;
}

export function formatDate(value: string): string {
  return new Intl.DateTimeFormat('pt-BR', { timeZone }).format(new Date(`${value}T12:00:00Z`));
}

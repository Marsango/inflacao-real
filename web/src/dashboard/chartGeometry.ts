import type { Observation } from '../data/model';
import type { ChartSize } from './useChartSize';

export function geometry(points: Observation[], size: ChartSize) {
  const plot = { left: 78, right: size.width - 16, top: 16, bottom: size.height - 32 };
  const values = points.flatMap((point) => point.value === null ? [] : [point.value]);
  const minimum = Math.min(...values);
  const maximum = Math.max(...values);
  const padding = Math.max((maximum - minimum) * 0.2, 10);
  const low = minimum - padding;
  const high = maximum + padding;
  const start = Date.parse(points[0].date);
  const duration = Math.max(Date.parse(points.at(-1)!.date) - start, 1);
  const x = (date: string) => plot.left + (Date.parse(date) - start) / duration * (plot.right - plot.left);
  const y = (value: number) => plot.bottom - (value - low) / (high - low) * (plot.bottom - plot.top);
  return { x, y, plot, ticks: Array.from({ length: 4 }, (_, index) => low + (high - low) * index / 3) };
}

export function lineSegments(points: Observation[], x: (date: string) => number, y: (value: number) => number): string {
  let connected = false;
  return points.map((point) => {
    if (point.value === null) { connected = false; return ''; }
    const command = connected ? 'L' : 'M';
    connected = true;
    return `${command}${x(point.date)},${y(point.value)}`;
  }).join(' ');
}

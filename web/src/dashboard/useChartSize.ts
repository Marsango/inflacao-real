import { useEffect, useRef, useState } from 'react';

export type ChartSize = { width: number; height: number };

export function useChartSize() {
  const container = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState<ChartSize>({ width: 1000, height: 360 });
  useEffect(() => {
    if (!container.current || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      if (width > 0 && height > 0) setSize({ width, height });
    });
    observer.observe(container.current);
    return () => observer.disconnect();
  }, []);
  return { container, size };
}

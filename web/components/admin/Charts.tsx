"use client";

import { useId } from "react";

type Point = {
  label: string;
  value: number;
};

export function SparkAreaChart({ title, points, color = "#22c55e" }: { title: string; points: Point[]; color?: string }) {
  const gradientID = useId().replace(/:/g, "");
  const width = 360;
  const height = 120;
  const max = Math.max(...points.map((point) => point.value), 1);
  const step = points.length > 1 ? width / (points.length - 1) : width;
  const line = points
    .map((point, index) => {
      const x = index * step;
      const y = height - (point.value / max) * (height - 18) - 8;
      return `${x},${y}`;
    })
    .join(" ");
  const area = `0,${height} ${line} ${width},${height}`;

  return (
    <figure className="rounded-lg border border-white/10 bg-white/[0.035] p-4">
      <figcaption className="mb-3 flex items-center justify-between gap-3">
        <span className="text-sm font-semibold text-zinc-100">{title}</span>
        <span className="font-mono text-xs text-zinc-400">{max.toLocaleString("ru-RU")} max</span>
      </figcaption>
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={title} className="h-32 w-full">
        <defs>
          <linearGradient id={gradientID} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.38" />
            <stop offset="100%" stopColor={color} stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <polyline points={area} fill={`url(#${gradientID})`} />
        <polyline points={line} fill="none" stroke={color} strokeLinecap="round" strokeLinejoin="round" strokeWidth="3" />
        {points.map((point, index) => {
          const x = index * step;
          const y = height - (point.value / max) * (height - 18) - 8;
          return <circle key={`${point.label}-${index}`} cx={x} cy={y} r="3" fill={color} />;
        })}
      </svg>
      <div className="mt-2 grid grid-cols-4 gap-2 text-xs text-zinc-500">
        {points.slice(-4).map((point) => (
          <span key={point.label}>{point.label}: {point.value.toLocaleString("ru-RU")}</span>
        ))}
      </div>
    </figure>
  );
}

export function UtilizationBars({ items }: { items: Array<{ name: string; utilization?: number; active?: boolean }> }) {
  return (
    <div className="rounded-lg border border-white/10 bg-white/[0.035] p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-zinc-100">Нагрузка VPS</h3>
        <span className="font-mono text-xs text-zinc-500">utilization</span>
      </div>
      <div className="space-y-3">
        {items.slice(0, 6).map((server) => {
          const value = Math.max(0, Math.min(100, Math.round(server.utilization ?? 0)));
          const color = value > 85 ? "bg-red-400" : value > 70 ? "bg-amber-300" : "bg-emerald-400";
          return (
            <div key={server.name} className="grid grid-cols-[minmax(90px,1fr)_minmax(120px,2fr)_42px] items-center gap-3 text-xs">
              <span className="truncate text-zinc-300">{server.name}</span>
              <span className="h-2 overflow-hidden rounded-full bg-white/10">
                <span className={`block h-full rounded-full ${color}`} style={{ width: `${value}%` }} />
              </span>
              <span className="text-right font-mono text-zinc-400">{value}%</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

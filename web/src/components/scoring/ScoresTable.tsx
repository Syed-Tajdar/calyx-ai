'use client';

import Link from 'next/link';
import type { ScoreRecord } from '@/lib/api';

interface ScoresTableProps {
  scores: ScoreRecord[];
  compact?: boolean;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function scoreColor(score: number): string {
  if (score >= 70) return 'text-emerald-400';
  if (score >= 50) return 'text-amber-400';
  return 'text-red-400';
}

export function ScoresTable({ scores, compact = false }: ScoresTableProps): JSX.Element {
  return (
    <div className="loom-card overflow-hidden">
      <div className="border-b border-loom-border px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">
          Call Quality
        </p>
        <h3 className="text-sm font-semibold text-white">
          {compact ? 'Recent Scores' : 'All Scored Calls'}
        </h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[500px] text-left text-xs">
          <thead>
            <tr className="border-b border-loom-border text-[10px] uppercase tracking-wider text-loom-muted">
              <th className="px-4 py-3 font-medium">Time</th>
              <th className="px-4 py-3 font-medium">Call ID</th>
              <th className="px-4 py-3 font-medium">Platform</th>
              <th className="px-4 py-3 font-medium">Score</th>
              <th className="px-4 py-3 font-medium">Sentiment</th>
            </tr>
          </thead>
          <tbody>
            {scores.map((s) => (
              <tr key={`${s.call_id}-${s.created_at}`} className="border-b border-loom-border/40">
                <td className="px-4 py-3 text-loom-muted">{formatTime(s.created_at)}</td>
                <td className="px-4 py-3">
                  <Link
                    href={`/calls/${encodeURIComponent(s.call_id)}`}
                    className="font-mono text-[10px] text-loom-accent hover:underline"
                  >
                    {s.call_id.slice(0, 12)}…
                  </Link>
                </td>
                <td className="px-4 py-3 capitalize text-loom-muted">{s.platform}</td>
                <td className={`px-4 py-3 font-semibold ${scoreColor(s.overall_score)}`}>
                  {s.overall_score.toFixed(1)}
                </td>
                <td className="px-4 py-3 capitalize text-white">{s.sentiment}</td>
              </tr>
            ))}
            {scores.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-12 text-center text-loom-muted">
                  No scored calls yet. Send a ping to trigger mock scoring.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

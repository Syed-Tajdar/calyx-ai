'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { ScorecardEditorModal } from '@/components/scoring/ScorecardEditorModal';
import { Button } from '@/components/ui/button';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import {
  ApiError,
  createScorecard,
  deleteScorecard,
  fetchScorecards,
  ScorecardRecord,
  updateScorecard,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

const PAGE_SIZE = 25;

export default function ScorecardsPage(): JSX.Element {
  const router = useRouter();
  const [scorecards, setScorecards] = useState<ScorecardRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<ScorecardRecord | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    setLoading(true);
    fetchScorecards(token, PAGE_SIZE, offset)
      .then((res) => {
        setScorecards(res.data);
        setTotal(res.total);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load scorecards');
      })
      .finally(() => setLoading(false));
  }, [router, offset]);

  async function handleSave(payload: {
    name: string;
    vertical: string;
    rules: ScorecardRecord['rules'];
  }): Promise<void> {
    const token = getToken();
    if (!token) return;
    if (editing) {
      const updated = await updateScorecard(token, editing.id, payload);
      setScorecards((prev) => prev.map((s) => (s.id === updated.id ? updated : s)));
      toast.success('Scorecard updated');
    } else {
      const created = await createScorecard(token, payload);
      setScorecards((prev) => [created, ...prev]);
      setTotal((t) => t + 1);
      toast.success('Scorecard created');
    }
  }

  async function handleDelete(card: ScorecardRecord): Promise<void> {
    if (!window.confirm(`Delete scorecard "${card.name}"?`)) return;
    const token = getToken();
    if (!token) return;
    setDeletingId(card.id);
    try {
      await deleteScorecard(token, card.id);
      setScorecards((prev) => prev.filter((s) => s.id !== card.id));
      setTotal((t) => Math.max(0, t - 1));
      toast.success('Scorecard removed');
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 404) {
        setScorecards((prev) => prev.filter((s) => s.id !== card.id));
        toast.message('Scorecard was already removed');
      } else {
        toast.error('Failed to delete scorecard');
      }
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Scorecards"
        description="Configure QA scorecards and weighted rules per vertical."
        action={
          <div className="flex gap-2">
            <Button type="button" variant="outline" size="sm" asChild>
              <Link href="/scoring">Back to Scoring</Link>
            </Button>
            <Button
              type="button"
              size="sm"
              className="gap-1"
              onClick={() => {
                setEditing(null);
                setEditorOpen(true);
              }}
            >
              <Plus className="h-4 w-4" />
              New scorecard
            </Button>
          </div>
        }
      />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="calyx-glass calyx-glass-lit overflow-hidden">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-calyx-border text-xs uppercase text-calyx-muted">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Vertical</th>
                <th className="px-4 py-3 numeric">Rules</th>
                <th className="px-4 py-3">Updated</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {scorecards.map((card) => (
                <tr key={card.id} className="border-b border-calyx-border/50 hover:bg-white/[0.03]">
                  <td className="px-4 py-3 font-medium text-calyx-text">{card.name}</td>
                  <td className="px-4 py-3 capitalize text-calyx-muted">{card.vertical}</td>
                  <td className="px-4 py-3 tabular-nums">{card.rules.length}</td>
                  <td className="px-4 py-3 text-calyx-muted">
                    {new Date(card.updated_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-1">
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="h-8 w-8"
                        onClick={() => {
                          setEditing(card);
                          setEditorOpen(true);
                        }}
                        aria-label={`Edit ${card.name}`}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="h-8 w-8 text-calyx-danger"
                        disabled={deletingId === card.id}
                        onClick={() => void handleDelete(card)}
                        aria-label={`Delete ${card.name}`}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {scorecards.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-12 text-center text-calyx-muted">
                    No scorecards yet. Create one to get started.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <ScorecardEditorModal
        open={editorOpen}
        initial={editing}
        onClose={() => {
          setEditorOpen(false);
          setEditing(null);
        }}
        onSave={handleSave}
      />
    </div>
  );
}

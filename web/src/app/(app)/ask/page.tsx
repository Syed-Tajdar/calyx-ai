'use client';

import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { History, X } from 'lucide-react';
import { ChatWindow } from '@/components/ask/ChatWindow';
import { SuggestedQuestions } from '@/components/ask/SuggestedQuestions';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ErrorBanner } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { ApiError, AskHistoryRecord, fetchAskHistory, postAsk } from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function AskPage(): JSX.Element {
  const router = useRouter();
  const [history, setHistory] = useState<AskHistoryRecord[]>([]);
  const [asking, setAsking] = useState(false);
  const [error, setError] = useState('');
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    fetchAskHistory(token, 20)
      .then((res) => setHistory(res.data ?? []))
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
        }
      });
  }, [router]);

  const handleAsk = useCallback(
    async (question: string) => {
      const token = getToken();
      if (!token) {
        router.replace('/login');
        throw new Error('not authenticated');
      }
      setAsking(true);
      setError('');
      try {
        const res = await postAsk(token, question);
        setHistory((prev) => [
          {
            id: res.query_id,
            question: res.question,
            answer: res.answer,
            intent: res.intent,
            cache_hit: res.cache_hit,
            row_count: res.row_count,
            tokens_used: res.tokens_used,
            created_at: new Date().toISOString(),
          },
          ...prev,
        ]);
        return res;
      } catch (err: unknown) {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
        }
        setError('Failed to get an answer. Try again.');
        throw err;
      } finally {
        setAsking(false);
      }
    },
    [router],
  );

  function HistoryList({ compact = false }: { compact?: boolean }): JSX.Element {
    if (history.length === 0) {
      return <p className="text-sm text-calyx-muted">No questions yet.</p>;
    }
    return (
      <ul className={compact ? 'space-y-2' : 'space-y-3'}>
        {history.map((h) => (
          <li key={h.id} className="rounded-lg border border-calyx-border bg-calyx-surface p-3">
            <p className="line-clamp-2 text-sm text-calyx-text">{h.question}</p>
            <div className="mt-2 flex flex-wrap gap-1">
              <Badge variant={h.cache_hit ? 'secondary' : 'default'}>
                {h.cache_hit ? 'cached' : h.intent}
              </Badge>
            </div>
          </li>
        ))}
      </ul>
    );
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Ask AI"
        description="Natural-language reporting over your ping and revenue data."
      />
      <IntelligenceSubNav />

      {error && <ErrorBanner message={error} />}

      <div className="mb-3 lg:hidden">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => setDrawerOpen(true)}
          className="gap-2"
        >
          <History className="h-4 w-4" />
          History ({history.length})
        </Button>
      </div>

      {drawerOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            className="absolute inset-0 bg-black/60"
            aria-label="Close history"
            onClick={() => setDrawerOpen(false)}
          />
          <div className="absolute bottom-0 left-0 right-0 max-h-[70vh] overflow-y-auto rounded-t-xl border border-calyx-border bg-calyx-card p-4">
            <div className="mb-3 flex items-center justify-between">
              <p className="text-sm font-semibold text-calyx-text">Question history</p>
              <Button type="button" variant="ghost" size="icon" onClick={() => setDrawerOpen(false)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <HistoryList compact />
          </div>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[1fr_280px]">
        <div className="space-y-4">
          <SuggestedQuestions
            disabled={asking}
            onSelect={(q) => {
              void handleAsk(q);
            }}
          />
          <ChatWindow history={history} onAsk={handleAsk} loading={asking} />
        </div>
        <aside className="hidden lg:block">
          <div className="calyx-card p-4">
            <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-calyx-muted">
              History
            </p>
            <HistoryList />
          </div>
        </aside>
      </div>
    </div>
  );
}

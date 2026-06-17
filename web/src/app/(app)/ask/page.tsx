'use client';

import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { ChatWindow } from '@/components/ask/ChatWindow';
import { SuggestedQuestions } from '@/components/ask/SuggestedQuestions';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { ErrorBanner } from '@/components/ui/PageStatus';
import { ApiError, AskHistoryRecord, fetchAskHistory, postAsk } from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function AskPage(): JSX.Element {
  const router = useRouter();
  const [history, setHistory] = useState<AskHistoryRecord[]>([]);
  const [asking, setAsking] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    fetchAskHistory(token, 10)
      .then((res) => setHistory(res.data))
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

  return (
    <>
      <AppHeader />
      <div className="mb-3 flex items-center justify-between">
        <SectionHeader title="Ask AI" />
        <Link href="/intelligence" className="text-xs text-loom-accent hover:underline">
          Intelligence hub
        </Link>
      </div>

      {error && <ErrorBanner message={error} />}

      <div className="mb-3">
        <SuggestedQuestions
          disabled={asking}
          onSelect={(q) => {
            void handleAsk(q);
          }}
        />
      </div>

      <ChatWindow history={history} onAsk={handleAsk} loading={asking} />
    </>
  );
}

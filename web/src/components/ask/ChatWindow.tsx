'use client';

import type { AskHistoryRecord, AskResponse } from '@/lib/api';

interface ChatWindowProps {
  history: AskHistoryRecord[];
  onAsk: (question: string) => Promise<AskResponse>;
  loading: boolean;
}

export function ChatWindow({ history, onAsk, loading }: ChatWindowProps): JSX.Element {
  async function handleSubmit(e: React.FormEvent<HTMLFormElement>): Promise<void> {
    e.preventDefault();
    const form = e.currentTarget;
    const input = form.elements.namedItem('question') as HTMLInputElement;
    const trimmed = input.value.trim();
    if (!trimmed || loading) return;
    input.value = '';
    await onAsk(trimmed);
  }

  const messages = history
    .slice()
    .reverse()
    .flatMap((h) => [
      { role: 'user' as const, text: h.question, meta: undefined },
      {
        role: 'assistant' as const,
        text: h.answer,
        meta: h.cache_hit ? 'cached' : h.intent,
      },
    ]);

  return (
    <div className="loom-card flex flex-col overflow-hidden">
      <div className="max-h-96 space-y-3 overflow-y-auto p-4">
        {messages.length === 0 && (
          <p className="text-sm text-loom-muted">
            Ask about revenue, pings, publishers, states, or compliance exposure.
          </p>
        )}
        {messages.map((msg, i) => (
          <div
            key={`${msg.role}-${i}-${msg.text.slice(0, 20)}`}
            className={`rounded-lg px-3 py-2 text-sm ${
              msg.role === 'user'
                ? 'ml-8 bg-loom-accent/15 text-white'
                : 'mr-8 bg-loom-surface text-loom-muted'
            }`}
          >
            <p className="whitespace-pre-wrap">{msg.text}</p>
            {msg.meta && (
              <p className="mt-1 text-[10px] uppercase tracking-wide text-loom-accent/80">{msg.meta}</p>
            )}
          </div>
        ))}
        {loading && <p className="text-xs text-loom-muted">Thinking…</p>}
      </div>
      <form onSubmit={handleSubmit} className="border-t border-loom-border p-3">
        <div className="flex gap-2">
          <input
            name="question"
            type="text"
            placeholder="e.g. What is my total revenue?"
            maxLength={2000}
            disabled={loading}
            className="flex-1 rounded-lg border border-loom-border bg-loom-bg px-3 py-2 text-sm text-white placeholder:text-loom-muted focus:border-loom-accent focus:outline-none disabled:opacity-60"
          />
          <button
            type="submit"
            disabled={loading}
            className="rounded-lg bg-loom-accent px-4 py-2 text-xs font-bold uppercase tracking-wide text-loom-bg transition hover:opacity-90 disabled:opacity-40"
          >
            Ask
          </button>
        </div>
      </form>
    </div>
  );
}

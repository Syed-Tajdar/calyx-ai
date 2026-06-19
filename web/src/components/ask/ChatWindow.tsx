'use client';

import type { AskHistoryRecord, AskResponse } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

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
      { role: 'user' as const, text: h.question, meta: undefined as string | undefined },
      {
        role: 'assistant' as const,
        text: h.answer,
        meta: h.cache_hit ? 'cached' : h.intent,
      },
    ]);

  return (
    <div className="calyx-card flex flex-col overflow-hidden">
      <div className="max-h-[28rem] space-y-3 overflow-y-auto p-4" aria-live="polite">
        {messages.length === 0 && (
          <p className="text-sm text-calyx-muted">
            Ask about revenue, pings, publishers, states, or compliance exposure.
          </p>
        )}
        {messages.map((msg, i) => (
          <div
            key={`${msg.role}-${i}-${msg.text.slice(0, 20)}`}
            className={`rounded-lg px-3 py-2 text-sm ${
              msg.role === 'user'
                ? 'ml-8 bg-calyx-accent-muted text-calyx-text'
                : 'mr-8 border border-calyx-border bg-calyx-card text-calyx-muted'
            }`}
          >
            <p className="whitespace-pre-wrap">{msg.text}</p>
            {msg.meta && (
              <Badge variant={msg.meta === 'cached' ? 'secondary' : 'default'} className="mt-2">
                {msg.meta}
              </Badge>
            )}
          </div>
        ))}
        {loading && <p className="text-xs text-calyx-muted">Thinking…</p>}
      </div>
      <form onSubmit={handleSubmit} className="border-t border-calyx-border p-3">
        <div className="flex gap-2">
          <Input
            name="question"
            type="text"
            placeholder="e.g. What is my total revenue?"
            maxLength={2000}
            disabled={loading}
            className="flex-1"
          />
          <Button type="submit" disabled={loading}>
            Ask
          </Button>
        </div>
      </form>
    </div>
  );
}

interface SuggestedQuestionsProps {
  onSelect: (question: string) => void;
  disabled?: boolean;
}

const SUGGESTIONS = [
  'Which publisher had the lowest conversion yesterday?',
  'Texas revenue this week vs last week',
  'Which campaigns dropped in the last 7 days?',
  'Revenue by platform in the last 30 days',
] as const;

export function SuggestedQuestions({ onSelect, disabled }: SuggestedQuestionsProps): JSX.Element {
  return (
    <div className="flex flex-wrap gap-2">
      {SUGGESTIONS.map((q) => (
        <button
          key={q}
          type="button"
          disabled={disabled}
          onClick={() => onSelect(q)}
          className="rounded-full border border-calyx-border bg-calyx-surface px-3 py-1.5 text-xs text-calyx-muted transition hover:border-calyx-accent hover:bg-calyx-accent-muted hover:text-calyx-text disabled:opacity-40"
        >
          {q}
        </button>
      ))}
    </div>
  );
}

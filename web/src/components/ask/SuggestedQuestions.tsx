interface SuggestedQuestionsProps {
  onSelect: (question: string) => void;
  disabled?: boolean;
}

const SUGGESTIONS = [
  'What is my total revenue?',
  'How many pings do I have?',
  'Which publisher has the highest revenue?',
  'What is my TCPA exposure?',
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
          className="rounded-full border border-loom-border px-3 py-1 text-xs text-loom-muted transition hover:border-loom-accent hover:text-white disabled:opacity-40"
        >
          {q}
        </button>
      ))}
    </div>
  );
}

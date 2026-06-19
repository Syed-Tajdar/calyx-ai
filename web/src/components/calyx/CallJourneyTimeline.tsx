export type JourneyStepState = 'complete' | 'active' | 'pending' | 'risk';

export interface JourneyStep {
  id: string;
  label: string;
  state: JourneyStepState;
}

interface CallJourneyTimelineProps {
  steps: JourneyStep[];
  className?: string;
}

const STATE_STYLES: Record<JourneyStepState, string> = {
  complete: 'border-calyx-teal bg-calyx-teal/20 text-calyx-teal',
  active: 'border-calyx-accent bg-calyx-accent-muted text-calyx-accent',
  pending: 'border-calyx-border bg-calyx-surface text-calyx-muted',
  risk: 'border-calyx-danger bg-calyx-danger/10 text-calyx-danger',
};

export function CallJourneyTimeline({ steps, className = '' }: CallJourneyTimelineProps): JSX.Element {
  return (
    <div className={`calyx-card p-4 ${className}`}>
      <p className="mb-4 text-[11px] font-semibold uppercase tracking-wider text-calyx-muted">
        Call journey
      </p>
      <ol className="flex flex-wrap items-center gap-2 sm:gap-0">
        {steps.map((step, index) => (
          <li key={step.id} className="flex items-center">
            <div className="flex flex-col items-center">
              <div
                className={`flex h-8 w-8 items-center justify-center rounded-full border-2 text-xs font-bold ${STATE_STYLES[step.state]}`}
              >
                {index + 1}
              </div>
              <span className="mt-1.5 max-w-[72px] text-center text-[10px] font-medium text-calyx-muted">
                {step.label}
              </span>
            </div>
            {index < steps.length - 1 && (
              <div
                className={`mx-1 hidden h-0.5 w-6 sm:block md:w-10 ${
                  step.state === 'complete' || step.state === 'risk'
                    ? 'bg-calyx-teal/50'
                    : 'bg-calyx-border'
                }`}
                aria-hidden
              />
            )}
          </li>
        ))}
      </ol>
    </div>
  );
}

export function buildJourneySteps(data: {
  hasPing: boolean;
  hasEnrichment: boolean;
  hasScore: boolean;
  hasCompliance: boolean;
  complianceRisk?: number;
  hasAttribution: boolean;
}): JourneyStep[] {
  const complianceState: JourneyStepState =
    data.hasCompliance && (data.complianceRisk ?? 0) >= 70
      ? 'risk'
      : data.hasCompliance
        ? 'complete'
        : data.hasScore
          ? 'active'
          : 'pending';

  return [
    {
      id: 'ping',
      label: 'Ping',
      state: data.hasPing ? 'complete' : 'pending',
    },
    {
      id: 'enrich',
      label: 'Enrich',
      state: data.hasEnrichment ? 'complete' : data.hasPing ? 'active' : 'pending',
    },
    {
      id: 'score',
      label: 'Score',
      state: data.hasScore ? 'complete' : data.hasEnrichment ? 'active' : 'pending',
    },
    {
      id: 'compliance',
      label: 'Compliance',
      state: complianceState,
    },
    {
      id: 'attribution',
      label: 'Attribution',
      state: data.hasAttribution ? 'complete' : complianceState === 'complete' ? 'active' : 'pending',
    },
  ];
}

interface LogoProps {
  className?: string;
  variant?: 'full' | 'compact';
}

export function Logo({ className = '', variant = 'full' }: LogoProps): JSX.Element {
  return (
    <div className={`flex items-center gap-2.5 ${className}`}>
      <svg
        width="36"
        height="36"
        viewBox="0 0 40 40"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden
      >
        <path
          d="M22 8L32 20L22 32"
          stroke="#00c9b7"
          strokeWidth="4"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M14 12L22 20L14 28"
          stroke="white"
          strokeWidth="3"
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity="0.9"
        />
      </svg>
      <div className="leading-none">
        <span className="text-lg font-semibold tracking-tight">
          <span className="text-white">Calyx </span>
          <span className="text-loom-accent">AI</span>
        </span>
        {variant === 'full' && (
          <span className="mt-0.5 block text-[10px] font-semibold uppercase tracking-[0.22em] text-loom-muted">
            Analytics
          </span>
        )}
      </div>
    </div>
  );
}

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
        <circle cx="20" cy="20" r="18" stroke="#6366F1" strokeWidth="1.5" opacity="0.4" />
        <path
          d="M20 8C14 12 12 18 12 22C12 28 16 32 20 32C24 32 28 28 28 22C28 18 26 12 20 8Z"
          fill="#00C9B7"
          fillOpacity="0.9"
        />
        <path
          d="M20 14C17 16.5 16 19 16 22C16 25 17.5 27 20 27C22.5 27 24 25 24 22C24 19 23 16.5 20 14Z"
          fill="#060B14"
          fillOpacity="0.35"
        />
      </svg>
      <div className="leading-none">
        <span className="text-lg font-semibold tracking-tight">
          <span className="text-calyx-text">Calyx </span>
          <span className="text-calyx-accent">AI</span>
        </span>
        {variant === 'full' && (
          <span className="mt-0.5 block text-[10px] font-semibold uppercase tracking-[0.22em] text-calyx-teal">
            Call Intelligence
          </span>
        )}
      </div>
    </div>
  );
}

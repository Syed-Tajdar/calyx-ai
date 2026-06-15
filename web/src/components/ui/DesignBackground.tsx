export function DesignBackground(): JSX.Element {
  return (
    <>
      <div
        className="pointer-events-none fixed inset-0 bg-loom-grid bg-grid opacity-70"
        aria-hidden
      />
      <div
        className="pointer-events-none fixed -bottom-40 -left-40 h-[500px] w-[500px] rounded-full bg-loom-accent/[0.07] blur-3xl"
        aria-hidden
      />
      <svg
        className="pointer-events-none fixed bottom-0 left-0 h-64 w-64 text-loom-accent/10"
        viewBox="0 0 200 200"
        fill="none"
        aria-hidden
      >
        <path d="M0 180 Q80 120 160 200" stroke="currentColor" strokeWidth="2" />
        <path d="M20 200 Q100 140 180 180" stroke="currentColor" strokeWidth="1.5" />
        <polygon points="40,160 60,120 80,160" fill="currentColor" opacity="0.5" />
      </svg>
    </>
  );
}

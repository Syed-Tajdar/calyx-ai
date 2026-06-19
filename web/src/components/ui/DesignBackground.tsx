export function DesignBackground(): JSX.Element {
  return (
    <>
      <div
        className="pointer-events-none fixed inset-0 bg-gradient-to-br from-calyx-bg via-calyx-bg to-[#0a1628]"
        aria-hidden
      />
      <div
        className="pointer-events-none fixed -right-32 top-0 h-96 w-96 rounded-full bg-calyx-accent/[0.06] blur-3xl"
        aria-hidden
      />
      <div
        className="pointer-events-none fixed -bottom-32 -left-32 h-96 w-96 rounded-full bg-calyx-teal/[0.05] blur-3xl"
        aria-hidden
      />
    </>
  );
}

export function DesignBackground(): JSX.Element {
  return (
    <>
      <div
        className="pointer-events-none fixed inset-0 bg-gradient-to-br from-calyx-bg via-calyx-bg to-[#051a1a]"
        aria-hidden
      />
      <div
        className="calyx-aurora-orb pointer-events-none fixed -right-24 top-[-6rem] h-[28rem] w-[28rem] rounded-full opacity-30"
        style={{ background: 'var(--aurora-orb-1)' }}
        aria-hidden
      />
      <div
        className="calyx-aurora-orb pointer-events-none fixed -bottom-32 -left-24 h-[26rem] w-[26rem] rounded-full opacity-25"
        style={{ background: 'var(--aurora-orb-2)' }}
        aria-hidden
      />
      <div
        className="calyx-aurora-orb pointer-events-none fixed left-1/3 top-1/2 h-48 w-48 rounded-full opacity-[0.08]"
        style={{ background: 'var(--aurora-orb-3)' }}
        aria-hidden
      />
    </>
  );
}

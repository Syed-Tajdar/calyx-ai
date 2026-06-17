import Link from 'next/link';

const LINKS = [
  {
    href: '/ask',
    title: 'Ask AI',
    description: 'Natural language analytics over your call data',
    accent: 'from-violet-500/20 to-loom-accent/10',
  },
  {
    href: '/compliance',
    title: 'TCPA Compliance',
    description: 'Risk scores, violations, and liability exposure',
    accent: 'from-amber-500/20 to-red-500/10',
  },
  {
    href: '/attribution',
    title: 'Ad Attribution',
    description: 'Meta, Google, and TikTok conversion feedback status',
    accent: 'from-sky-500/20 to-emerald-500/10',
  },
] as const;

export function IntelligenceHubCards(): JSX.Element {
  return (
    <section className="grid gap-3 sm:grid-cols-3">
      {LINKS.map((link) => (
        <Link
          key={link.href}
          href={link.href}
          className={`loom-card block bg-gradient-to-br p-4 transition hover:border-loom-accent/50 ${link.accent}`}
        >
          <p className="text-sm font-semibold text-white">{link.title}</p>
          <p className="mt-1 text-xs leading-relaxed text-loom-muted">{link.description}</p>
          <span className="mt-3 inline-block text-[10px] font-bold uppercase tracking-wider text-loom-accent">
            Open →
          </span>
        </Link>
      ))}
    </section>
  );
}

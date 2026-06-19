import { Brain, Megaphone, Shield } from 'lucide-react';
import { IntelligenceCommandCard } from '@/components/calyx/IntelligenceCommandCard';

const LINKS = [
  {
    href: '/ask',
    title: 'Ask AI',
    description: 'Natural language analytics over your call data — revenue, publishers, compliance.',
    icon: Brain,
    accent: 'indigo' as const,
  },
  {
    href: '/compliance',
    title: 'TCPA Compliance',
    description: 'Risk scores, DNC matches, violations, and liability exposure in real time.',
    icon: Shield,
    accent: 'amber' as const,
  },
  {
    href: '/attribution',
    title: 'Ad Attribution',
    description: 'Meta, Google, and TikTok conversion feedback — fired and failed events.',
    icon: Megaphone,
    accent: 'teal' as const,
  },
] as const;

export function IntelligenceHubCards(): JSX.Element {
  return (
    <section className="grid gap-4 sm:grid-cols-3">
      {LINKS.map((link) => (
        <IntelligenceCommandCard
          key={link.href}
          href={link.href}
          title={link.title}
          description={link.description}
          icon={link.icon}
          accent={link.accent}
        />
      ))}
    </section>
  );
}

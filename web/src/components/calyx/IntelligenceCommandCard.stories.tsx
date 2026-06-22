import type { Meta, StoryObj } from '@storybook/react';
import { Brain } from 'lucide-react';
import { IntelligenceCommandCard } from '@/components/calyx/IntelligenceCommandCard';

const meta: Meta<typeof IntelligenceCommandCard> = {
  title: 'Calyx/IntelligenceCommandCard',
  component: IntelligenceCommandCard,
};

export default meta;
type Story = StoryObj<typeof IntelligenceCommandCard>;

export const Teal: Story = {
  args: {
    href: '/intelligence',
    title: 'Ping Intelligence',
    description: 'Search, dedup, and bid analysis across all connected platforms.',
    icon: Brain,
    accent: 'teal',
  },
};

export const Indigo: Story = {
  args: {
    href: '/compliance',
    title: 'TCPA Compliance',
    description: 'Risk scoring, DNC matches, and liability exposure tracking.',
    icon: Brain,
    accent: 'indigo',
  },
};

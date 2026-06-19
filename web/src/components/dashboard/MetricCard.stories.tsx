import type { Meta, StoryObj } from '@storybook/react';
import { DollarSign } from 'lucide-react';
import { MetricCard } from '@/components/dashboard/MetricCard';

const meta: Meta<typeof MetricCard> = {
  title: 'Calyx/MetricCard',
  component: MetricCard,
};

export default meta;
type Story = StoryObj<typeof MetricCard>;

export const Revenue: Story = {
  args: {
    label: 'Total Revenue',
    value: '$12,450.00',
    trend: 'RPC $4.20',
    trendUp: true,
    icon: DollarSign,
  },
};

import type { Meta, StoryObj } from '@storybook/react';
import { RiskBadge } from '@/components/ui/RiskBadge';

const meta: Meta<typeof RiskBadge> = {
  title: 'Calyx/RiskBadge',
  component: RiskBadge,
};

export default meta;
type Story = StoryObj<typeof RiskBadge>;

export const Low: Story = { args: { score: 25 } };
export const Medium: Story = { args: { score: 55 } };
export const High: Story = { args: { score: 85 } };

import type { Meta, StoryObj } from '@storybook/react';
import { ComplianceRiskRing } from '@/components/calyx/ComplianceRiskRing';

const meta: Meta<typeof ComplianceRiskRing> = {
  title: 'Calyx/ComplianceRiskRing',
  component: ComplianceRiskRing,
};

export default meta;
type Story = StoryObj<typeof ComplianceRiskRing>;

export const MediumRisk: Story = {
  args: {
    score: 52,
    exposureUsd: 1250.5,
    size: 'md',
  },
};

export const HighRisk: Story = {
  args: {
    score: 82,
    exposureUsd: 4800,
    size: 'sm',
  },
};

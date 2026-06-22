import type { Meta, StoryObj } from '@storybook/react';
import { PlatformChart } from '@/components/dashboard/PlatformChart';

const meta: Meta<typeof PlatformChart> = {
  title: 'Calyx/PlatformChart',
  component: PlatformChart,
};

export default meta;
type Story = StoryObj<typeof PlatformChart>;

export const Default: Story = {
  args: {
    data: [
      { name: 'ringba', value: 480, percent: 48 },
      { name: 'google', value: 310, percent: 31 },
      { name: 'meta', value: 210, percent: 21 },
    ],
  },
};

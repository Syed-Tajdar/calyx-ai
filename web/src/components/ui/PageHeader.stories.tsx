import type { Meta, StoryObj } from '@storybook/react';
import { PageHeader } from '@/components/ui/PageHeader';
import { Button } from '@/components/ui/button';

const meta: Meta<typeof PageHeader> = {
  title: 'Calyx/PageHeader',
  component: PageHeader,
};

export default meta;
type Story = StoryObj<typeof PageHeader>;

export const Default: Story = {
  args: {
    title: 'Operations Overview',
    description: 'Real-time ping pipeline and financial metrics.',
  },
};

export const WithAction: Story = {
  args: {
    title: 'Ping Feed',
    description: 'RTB ping stream from connected platforms.',
    action: <Button size="sm">Export</Button>,
  },
};

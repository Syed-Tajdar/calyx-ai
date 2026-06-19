import type { Meta, StoryObj } from '@storybook/react';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';

const meta: Meta<typeof IntelligenceSubNav> = {
  title: 'Calyx/IntelligenceSubNav',
  component: IntelligenceSubNav,
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: '/intelligence',
      },
    },
  },
  decorators: [
    (Story) => (
      <div className="w-full max-w-xl">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof IntelligenceSubNav>;

export const Hub: Story = {
  render: () => <IntelligenceSubNav />,
};

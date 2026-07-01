import type { Meta, StoryObj } from '@storybook/react';
import { FilterBar } from '@/components/ui/FilterBar';
import { DateFilterProvider } from '@/lib/date-filter';

const meta: Meta<typeof FilterBar> = {
  title: 'Calyx/FilterBar',
  component: FilterBar,
  decorators: [
    (Story) => (
      <DateFilterProvider>
        <Story />
      </DateFilterProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof FilterBar>;

export const DateOnly: Story = {};

export const WithPlatform: Story = {
  args: {
    showPlatform: true,
    platforms: ['ringba', 'retreaver', 'callloom-native'],
  },
};

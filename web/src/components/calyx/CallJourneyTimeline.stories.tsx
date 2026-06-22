import type { Meta, StoryObj } from '@storybook/react';
import { CallJourneyTimeline } from '@/components/calyx/CallJourneyTimeline';

const meta: Meta<typeof CallJourneyTimeline> = {
  title: 'Calyx/CallJourneyTimeline',
  component: CallJourneyTimeline,
};

export default meta;
type Story = StoryObj<typeof CallJourneyTimeline>;

export const InProgress: Story = {
  args: {
    steps: [
      { id: 'ping', label: 'Ping', state: 'complete' },
      { id: 'enrich', label: 'Enrich', state: 'complete' },
      { id: 'score', label: 'Score', state: 'complete' },
      { id: 'comp', label: 'Compliance', state: 'active' },
      { id: 'attr', label: 'Attribution', state: 'pending' },
    ],
  },
};

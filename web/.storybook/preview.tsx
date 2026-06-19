import type { Preview } from '@storybook/react';
import React from 'react';
import '../src/app/globals.css';

const preview: Preview = {
  parameters: {
    backgrounds: {
      default: 'calyx-dark',
      values: [
        { name: 'calyx-dark', value: '#060B14' },
        { name: 'calyx-light', value: '#F9FAFB' },
      ],
    },
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div className="dark min-w-[320px] bg-calyx-bg p-6 text-calyx-text">
        <Story />
      </div>
    ),
  ],
};

export default preview;

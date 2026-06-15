/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        loom: {
          bg: '#071615',
          surface: '#0c2423',
          card: '#0f2e2c',
          border: '#1a4a47',
          accent: '#00c9b7',
          'accent-hover': '#00e5d4',
          muted: '#8ba8a6',
          teal: '#14b8a6',
        },
      },
      fontFamily: {
        sans: ['var(--font-inter)', 'system-ui', 'sans-serif'],
      },
      backgroundImage: {
        'loom-grid':
          'linear-gradient(rgba(0,201,183,0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(0,201,183,0.03) 1px, transparent 1px)',
      },
      backgroundSize: {
        grid: '48px 48px',
      },
      boxShadow: {
        'loom-glow': '0 0 80px rgba(0, 201, 183, 0.06)',
      },
    },
  },
  plugins: [],
};

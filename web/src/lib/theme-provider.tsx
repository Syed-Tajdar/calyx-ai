'use client';

import { createContext, useContext, useEffect, type ReactNode } from 'react';

type Theme = 'dark';

interface ThemeContextValue {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function applyTheme(): void {
  document.documentElement.classList.remove('light');
  document.documentElement.classList.add('dark');
}

export function ThemeProvider({ children }: { children: ReactNode }): JSX.Element {
  useEffect(() => {
    applyTheme();
    localStorage.removeItem('calyx-theme');
  }, []);

  const value: ThemeContextValue = {
    theme: 'dark',
    setTheme: () => applyTheme(),
    toggleTheme: () => applyTheme(),
  };

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within ThemeProvider');
  }
  return ctx;
}

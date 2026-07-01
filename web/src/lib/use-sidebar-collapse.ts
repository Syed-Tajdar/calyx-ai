'use client';

import { useCallback, useEffect, useState } from 'react';

const STORAGE_KEY = 'calyx-sidebar-collapsed';

export function useSidebarCollapse(): {
  collapsed: boolean;
  toggle: () => void;
  ready: boolean;
} {
  const [collapsed, setCollapsed] = useState(false);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored === 'true') setCollapsed(true);
    } catch {
      // ignore storage errors
    }
    setReady(true);
  }, []);

  const toggle = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(STORAGE_KEY, String(next));
      } catch {
        // ignore storage errors
      }
      return next;
    });
  }, []);

  return { collapsed, toggle, ready };
}

'use client';

import { useCallback, useEffect, useRef, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  /** Body content — form fields, a summary of what will change, warnings. */
  children?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  /** Blocks confirm (e.g. invalid amount) without hiding the reason from the operator. */
  confirmDisabled?: boolean;
  /** Renders the confirm action in danger styling (used for Cancel conversion). */
  destructive?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Modal confirm used for write-back actions, which change money in the connected platform.
 *
 * Deliberately written in-repo rather than pulling in a dialog dependency: the app ships no other
 * modal, and the security baseline keeps the bundle and CSP surface small. Handles focus trapping,
 * Escape, backdrop click, scroll lock and `aria-modal` semantics.
 */
export function ConfirmDialog({
  open,
  title,
  children,
  confirmLabel,
  cancelLabel = 'Cancel',
  confirmDisabled = false,
  destructive = false,
  busy = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps): JSX.Element | null {
  const panelRef = useRef<HTMLDivElement>(null);
  const previouslyFocused = useRef<HTMLElement | null>(null);

  const requestCancel = useCallback(() => {
    if (busy) return; // never abandon an in-flight write
    onCancel();
  }, [busy, onCancel]);

  // Focus the first control on open; restore focus to the trigger on close.
  useEffect(() => {
    if (!open) return;
    previouslyFocused.current = document.activeElement as HTMLElement | null;
    const first = panelRef.current?.querySelector<HTMLElement>(FOCUSABLE);
    first?.focus();
    return () => previouslyFocused.current?.focus?.();
  }, [open]);

  // Escape to dismiss, Tab cycles inside the panel.
  useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent): void {
      if (event.key === 'Escape') {
        event.preventDefault();
        requestCancel();
        return;
      }
      if (event.key !== 'Tab') return;
      const nodes = panelRef.current?.querySelectorAll<HTMLElement>(FOCUSABLE);
      if (!nodes || nodes.length === 0) return;
      const first = nodes[0];
      const last = nodes[nodes.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open, requestCancel]);

  // Prevent the page behind the dialog from scrolling.
  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previous;
    };
  }, [open]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) requestCancel();
      }}
    >
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" aria-hidden="true" />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        className="calyx-glass relative w-full max-w-md rounded-xl border border-calyx-border bg-calyx-card p-5 shadow-2xl"
      >
        <h2
          id="confirm-dialog-title"
          className="font-heading text-base font-semibold text-calyx-text"
        >
          {title}
        </h2>

        {children && <div className="mt-4 space-y-3 text-sm text-calyx-muted">{children}</div>}

        <div className="mt-6 flex justify-end gap-2">
          <Button type="button" variant="outline" size="sm" onClick={requestCancel} disabled={busy}>
            {cancelLabel}
          </Button>
          <Button
            type="button"
            size="sm"
            variant={destructive ? 'secondary' : 'default'}
            className={
              destructive ? 'bg-calyx-danger text-white hover:bg-calyx-danger/90' : undefined
            }
            onClick={onConfirm}
            disabled={confirmDisabled || busy}
          >
            {busy ? 'Working…' : confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

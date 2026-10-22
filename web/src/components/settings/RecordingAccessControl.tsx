'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ApiError, clearRecordingAccess, setRecordingAccess } from '@/lib/api';
import { getToken } from '@/lib/auth';

interface Props {
  integrationId: string;
  isSet: boolean;
  onChanged: () => void;
}

/**
 * v2 Phase 2B — Ringba Recording Access credentials on an existing integration. Write-only: the
 * values are sent once, stored encrypted (like the API key) and never shown again; the list only
 * says whether they are set. Admin only (the API answers 403 otherwise).
 */
export function RecordingAccessControl({ integrationId, isSet, onChanged }: Props): JSX.Element {
  const [open, setOpen] = useState(false);
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [busy, setBusy] = useState(false);

  async function save(): Promise<void> {
    const token = getToken();
    if (!token || !user.trim() || !pass) return;
    setBusy(true);
    try {
      await setRecordingAccess(token, integrationId, user.trim(), pass);
      toast.success('Recording access saved');
      setOpen(false);
      setUser('');
      setPass('');
      onChanged();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Could not save the recording access');
    } finally {
      setBusy(false);
    }
  }

  async function clear(): Promise<void> {
    const token = getToken();
    if (!token || !window.confirm('Remove the Recording Access credentials? Older recordings can then no longer be fetched.')) return;
    setBusy(true);
    try {
      await clearRecordingAccess(token, integrationId);
      toast.success('Recording access removed');
      onChanged();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Could not remove the recording access');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-1 space-y-1 text-[10px]" data-testid="recording-access">
      <div className="flex items-center gap-2">
        <span className={isSet ? 'text-emerald-400' : 'text-calyx-muted'}>
          Recording access: {isSet ? 'set' : 'not set'}
        </span>
        <button type="button" className="underline" disabled={busy} onClick={() => setOpen((o) => !o)}>
          {isSet ? 'Replace' : 'Set'}
        </button>
        {isSet && (
          <button type="button" className="underline" disabled={busy} onClick={() => void clear()}>
            Clear
          </button>
        )}
      </div>
      {open && (
        <div className="flex flex-wrap items-center gap-1">
          <Input aria-label="Recording access username" className="h-7 w-36 text-xs" autoComplete="off" value={user}
            onChange={(e) => setUser(e.target.value)} placeholder="Username" />
          <Input aria-label="Recording access password" className="h-7 w-36 text-xs" type="password" autoComplete="new-password"
            value={pass} onChange={(e) => setPass(e.target.value)} placeholder="Password" />
          <Button type="button" size="sm" className="h-7" disabled={busy || !user.trim() || !pass} onClick={() => void save()}>
            Save
          </Button>
        </div>
      )}
    </div>
  );
}

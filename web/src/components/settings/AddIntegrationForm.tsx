'use client';



import { useEffect, useState } from 'react';

import { toast } from 'sonner';

import { Button } from '@/components/ui/button';

import { Input } from '@/components/ui/input';

import {

  ApiError,

  createIntegration,

  fetchPlatformConnectionOptions,

  type ApiSyncCredentialField,

  type CreateIntegrationInput,

  type IntegrationRecord,

  type PlatformConnectionOption,

  syncIntegration,

  testIntegration,

} from '@/lib/api';

import { getToken } from '@/lib/auth';



interface AddIntegrationFormProps {

  onCreated: (record: IntegrationRecord) => void;

}



export function AddIntegrationForm({ onCreated }: AddIntegrationFormProps): JSX.Element {

  const [options, setOptions] = useState<PlatformConnectionOption[]>([]);

  const [platform, setPlatform] = useState('ringba');

  const [name, setName] = useState('');

  const [connectionMode, setConnectionMode] = useState<'webhook' | 'api_sync'>('api_sync');

  const [webhookSecret, setWebhookSecret] = useState('');

  const [credValues, setCredValues] = useState<Record<string, string>>({});

  const [saving, setSaving] = useState(false);



  useEffect(() => {

    const token = getToken();

    if (!token) return;

    fetchPlatformConnectionOptions(token)

      .then(setOptions)

      .catch(() => toast.error('Failed to load platform options'));

  }, []);



  const selected = options.find((o) => o.platform === platform);

  const canWebhook = selected?.supports_webhook ?? true;

  const canApiSync = selected?.supports_api_sync ?? true;

  const apiFields: ApiSyncCredentialField[] =

    selected?.api_sync_fields ?? [{ key: 'api_key', label: 'API key', secret: true }];



  function setCred(key: string, value: string): void {

    setCredValues((prev) => ({ ...prev, [key]: value }));

  }



  async function handleSubmit(e: React.FormEvent): Promise<void> {

    e.preventDefault();

    const token = getToken();

    if (!token) return;

    if (!name.trim()) {

      toast.error('Name is required');

      return;

    }

    const body: CreateIntegrationInput = {

      platform,

      name: name.trim(),

      connection_mode: connectionMode,

    };

    if (connectionMode === 'webhook') {

      body.webhook_secret = webhookSecret.trim();

    } else {

      for (const field of apiFields) {

        const val = credValues[field.key]?.trim();

        if (!val) {

          toast.error(`${field.label} is required`);

          return;

        }

        body[field.key as keyof CreateIntegrationInput] = val as never;

      }

    }

    setSaving(true);

    try {

      const created = await createIntegration(token, body);

      onCreated(created);

      toast.success('Integration saved');

      setName('');

      setWebhookSecret('');

      setCredValues({});

      if (connectionMode === 'api_sync') {

        const test = await testIntegration(token, created.id);

        if (test.ok) {

          await syncIntegration(token, created.id);

          toast.success('First sync started');

        } else {

          toast.error(test.message);

        }

      }

    } catch (err: unknown) {

      if (err instanceof ApiError) {

        toast.error(err.message);

      } else {

        toast.error('Failed to save integration');

      }

    } finally {

      setSaving(false);

    }

  }



  return (

    <form

      onSubmit={(e) => void handleSubmit(e)}

      className="calyx-glass calyx-glass-lit space-y-4 p-4"

    >

      <div>

        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">

          Add integration

        </p>

        <h3 className="font-heading text-sm font-semibold text-calyx-text">

          Connect a platform

        </h3>

      </div>



      <div className="grid gap-3 sm:grid-cols-2">

        <div className="space-y-1.5">

          <label htmlFor="platform" className="mb-1 block text-xs text-calyx-muted">

            Platform

          </label>

          <select

            id="platform"

            className="flex h-9 w-full rounded-md border border-calyx-border bg-calyx-surface px-3 text-sm"

            value={platform}

            onChange={(e) => {

              setPlatform(e.target.value);

              setCredValues({});

            }}

          >

            {(options.length

              ? options

              : [{ platform: 'ringba', supports_webhook: true, supports_api_sync: true }]

            ).map((o) => (

              <option key={o.platform} value={o.platform}>

                {o.platform}

              </option>

            ))}

          </select>

        </div>

        <div className="space-y-1.5">

          <label htmlFor="integration-name" className="mb-1 block text-xs text-calyx-muted">

            Name

          </label>

          <Input

            id="integration-name"

            value={name}

            onChange={(e) => setName(e.target.value)}

            placeholder="My Retreaver account"

          />

        </div>

      </div>



      <div className="space-y-2">

        <p className="mb-2 text-xs text-calyx-muted">Connection type</p>

        <div className="flex flex-col gap-2 sm:flex-row">

          {canApiSync && (

            <label className="flex cursor-pointer items-center gap-2 rounded-md border border-calyx-border px-3 py-2 text-sm">

              <input

                type="radio"

                name="connection_mode"

                checked={connectionMode === 'api_sync'}

                onChange={() => setConnectionMode('api_sync')}

              />

              API key — no callback setup

            </label>

          )}

          {canWebhook && (

            <label className="flex cursor-pointer items-center gap-2 rounded-md border border-calyx-border px-3 py-2 text-sm">

              <input

                type="radio"

                name="connection_mode"

                checked={connectionMode === 'webhook'}

                onChange={() => setConnectionMode('webhook')}

              />

              Webhook — postback URL in platform

            </label>

          )}

        </div>

      </div>



      {connectionMode === 'api_sync' ? (

        <div className="space-y-3">

          {apiFields.map((field) => (

            <div key={field.key} className="space-y-1.5">

              <label htmlFor={`cred-${field.key}`} className="mb-1 block text-xs text-calyx-muted">

                {field.label}

              </label>

              <Input

                id={`cred-${field.key}`}

                type={field.secret ? 'password' : 'text'}

                autoComplete="off"

                value={credValues[field.key] ?? ''}

                onChange={(e) => setCred(field.key, e.target.value)}

                placeholder={field.placeholder ?? `Paste ${field.label.toLowerCase()}`}

              />

            </div>

          ))}

          <p className="text-[11px] text-calyx-muted">

            Calyx pulls calls on a schedule. You do not need to configure a webhook URL.

          </p>

        </div>

      ) : (

        <div className="space-y-1.5">

          <label htmlFor="webhook-secret" className="mb-1 block text-xs text-calyx-muted">

            Webhook signing secret

          </label>

          <Input

            id="webhook-secret"

            type="password"

            autoComplete="off"

            value={webhookSecret}

            onChange={(e) => setWebhookSecret(e.target.value)}

            placeholder={platform === 'trackdrive' ? 'Optional for TrackDrive' : 'HMAC secret'}

          />

          <p className="text-[11px] text-calyx-muted">

            After save, use the webhook token shown in the list for your postback URL.

          </p>

        </div>

      )}



      <Button type="submit" disabled={saving}>

        {saving ? 'Saving…' : 'Save integration'}

      </Button>

    </form>

  );

}



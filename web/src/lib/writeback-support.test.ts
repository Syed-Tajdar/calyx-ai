import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  platformSupportsWriteback,
  RETREAVER_PIXEL_WARNING,
  writebackCopy,
  writebackView,
  type WritebackIntegration,
} from './writeback-support.ts';

const OPTIONS = [
  { platform: 'ringba', supports_writeback: true },
  { platform: 'retreaver', supports_writeback: true },
  { platform: 'trackdrive', supports_writeback: false },
  { platform: 'callgrid', supports_writeback: false },
];

function live(platform: string, extra: Partial<WritebackIntegration> = {}): WritebackIntegration {
  return {
    id: `int-${platform}`,
    platform,
    connection_mode: 'api_sync',
    sync_mode: 'real',
    writeback_enabled: true,
    supports_writeback: true,
    ...extra,
  };
}

describe('platformSupportsWriteback (backend registry)', () => {
  it('follows supports_writeback from platform-options', () => {
    assert.equal(platformSupportsWriteback(OPTIONS, 'ringba'), true);
    assert.equal(platformSupportsWriteback(OPTIONS, 'retreaver'), true);
    assert.equal(platformSupportsWriteback(OPTIONS, 'trackdrive'), false);
    assert.equal(platformSupportsWriteback(OPTIONS, 'callgrid'), false);
  });

  it('is false until the backend has answered, and for an old backend without the field', () => {
    assert.equal(platformSupportsWriteback(null, 'retreaver'), false);
    assert.equal(platformSupportsWriteback([{ platform: 'retreaver' }], 'retreaver'), false);
  });
});

describe('writebackView', () => {
  const base = { canSee: true, platformLabel: 'Retreaver', options: OPTIONS };

  it('shows Convert/Cancel for Retreaver only when supported AND enabled for the tenant', () => {
    const v = writebackView({ ...base, platform: 'retreaver', integrations: [live('retreaver')] });
    assert.equal(v.kind, 'ready');
    const off = writebackView({
      ...base,
      platform: 'retreaver',
      integrations: [live('retreaver', { writeback_enabled: false })],
    });
    assert.deepEqual(off, { kind: 'unavailable', reason: 'Write-back is turned off for this account.' });
  });

  it('is hidden for a non-admin', () => {
    const v = writebackView({ ...base, canSee: false, platform: 'retreaver', integrations: [live('retreaver')] });
    assert.equal(v.kind, 'hidden');
  });

  it('is hidden for platforms the backend does not support', () => {
    for (const p of ['trackdrive', 'callgrid']) {
      assert.equal(writebackView({ ...base, platform: p, integrations: [live(p)] }).kind, 'hidden');
    }
    // Before platform-options has loaded nothing is shown, so an unsupported card never flashes.
    assert.equal(
      writebackView({ ...base, options: null, platform: 'retreaver', integrations: null }).kind,
      'hidden',
    );
  });

  it('is hidden when the integration itself says unsupported', () => {
    const v = writebackView({
      ...base,
      platform: 'retreaver',
      integrations: [live('retreaver', { supports_writeback: false })],
    });
    assert.equal(v.kind, 'hidden');
  });

  it('needs a live API-key integration — Test/paused (mock) is not enough', () => {
    const v = writebackView({
      ...base,
      platform: 'retreaver',
      integrations: [live('retreaver', { sync_mode: 'mock' })],
    });
    assert.deepEqual(v, {
      kind: 'unavailable',
      reason: 'No live Retreaver API connection — add one in Settings and set it to Live.',
    });
    assert.deepEqual(writebackView({ ...base, platform: 'retreaver', integrations: null }), {
      kind: 'unavailable',
      reason: 'Checking connection…',
    });
  });

  it('keeps the Ringba states unchanged', () => {
    const rb = { ...base, platformLabel: 'Ringba', platform: 'ringba' };
    assert.equal(writebackView({ ...rb, integrations: [live('ringba')] }).kind, 'ready');
    assert.deepEqual(writebackView({ ...rb, integrations: [] }), {
      kind: 'unavailable',
      reason: 'No live Ringba API connection — add one in Settings and set it to Live.',
    });
  });
});

describe('writebackCopy', () => {
  it('Retreaver: pixel warning in the convert popup and void wording', () => {
    const c = writebackCopy('retreaver', 'Retreaver');
    assert.equal(
      c.convertWarning,
      "Retreaver may fire this call's conversion pixels/postbacks to the buyer or publisher.",
    );
    assert.equal(c.convertWarning, RETREAVER_PIXEL_WARNING);
    assert.equal(c.cancelButton, 'Void conversion (sets revenue to $0)');
    assert.match(c.cancelBody, /revenue to \$0 in Retreaver/);
  });

  it('Ringba: the pre-4b wording, no pixel line', () => {
    const c = writebackCopy('ringba', 'Ringba');
    assert.equal(c.convertWarning, null);
    assert.equal(c.cancelButton, 'Cancel conversion');
    assert.equal(c.cancelTitle, 'Cancel this conversion');
    assert.equal(c.cancelConfirmLabel, 'Confirm cancel');
    assert.equal(c.cancelHeading, 'Cancel this conversion?');
    assert.equal(
      c.cancelBody,
      'This voids the conversion and its payout in Ringba. It is reversible — the call can be converted again afterwards.',
    );
  });
});

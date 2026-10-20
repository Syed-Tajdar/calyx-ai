import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { featureForRoute, isRouteVisible } from './v1-features.ts';
import {
  alertActionLabel,
  alertButton,
  alertStatusLabel,
  bellCountLabel,
  modeAvailability,
  pauseWarningText,
  pingsLinkFor,
  TRIGGER_OPTIONS,
  triggerLabel,
} from './workflows.ts';

const NOW = new Date('2026-10-20T15:00:00Z');
const daysAgo = (n: number) => new Date(NOW.getTime() - n * 86_400_000).toISOString();
const base = {
  trigger: 'conversion_rate_below',
  scope: 'publisher' as const,
  platform: 'retreaver' as const,
  savedMode: 'suggest' as const,
  suggestSince: daysAgo(8),
  ringbaVerified: false,
  now: NOW,
};
const modeOf = (list: ReturnType<typeof modeAvailability>, m: string) => list.find((x) => x.mode === m)!;

describe('mode availability (rule form)', () => {
  it('Auto is available after 7 days in Suggest on Retreaver', () => {
    assert.equal(modeOf(modeAvailability(base), 'auto').enabled, true);
  });

  it('Auto is disabled with a reason before 7 days of Suggest', () => {
    const a = modeOf(modeAvailability({ ...base, suggestSince: daysAgo(5) }), 'auto');
    assert.equal(a.enabled, false);
    assert.match(a.reason ?? '', /2 days left/);
  });

  it('a new rule (or a Notify rule) must be saved in Suggest first', () => {
    assert.match(modeOf(modeAvailability({ ...base, savedMode: null, suggestSince: null }), 'auto').reason ?? '', /Save the rule in Suggest/);
    assert.equal(modeOf(modeAvailability({ ...base, savedMode: 'notify', suggestSince: daysAgo(30) }), 'auto').enabled, false);
  });

  it('Auto is disabled for Ringba until the pause is verified', () => {
    const off = modeOf(modeAvailability({ ...base, platform: 'ringba' }), 'auto');
    assert.equal(off.enabled, false);
    assert.match(off.reason ?? '', /Ringba/);
    assert.equal(modeOf(modeAvailability({ ...base, platform: 'ringba', ringbaVerified: true }), 'auto').enabled, true);
  });

  it('integration_unhealthy and non-publisher scopes are notify only', () => {
    const unhealthy = modeAvailability({ ...base, trigger: 'integration_unhealthy' });
    assert.equal(modeOf(unhealthy, 'suggest').enabled, false);
    assert.equal(modeOf(unhealthy, 'auto').enabled, false);
    const campaign = modeAvailability({ ...base, scope: 'campaign' });
    assert.equal(modeOf(campaign, 'suggest').enabled, false);
    assert.equal(modeOf(campaign, 'notify').enabled, true);
  });
});

describe('labels', () => {
  it('Retreaver confirmed by response (owner Q10)', () => {
    assert.equal(alertActionLabel('paused', 'retreaver', { confirmation: 'response' }), "Paused (confirmed by Retreaver's response)");
    assert.equal(alertActionLabel('paused', 'ringba', { confirmation: 'read_back' }), 'Paused (read back confirmed)');
    assert.equal(alertActionLabel('pause_blocked', 'ringba'), 'Pause blocked');
  });

  it('statuses and triggers', () => {
    assert.equal(alertStatusLabel('acknowledged'), 'Acknowledged');
    assert.equal(alertStatusLabel('actioned'), 'Actioned');
    assert.equal(triggerLabel('no_calls'), 'Campaign has no calls for the window');
    assert.equal(TRIGGER_OPTIONS.filter((t) => t.qa).length, 4);
  });

  it('the confirm popup warning names the platform', () => {
    assert.equal(pauseWarningText('ringba'), 'Pausing stops this publisher on ALL campaigns on Ringba.');
    assert.equal(pauseWarningText('retreaver'), 'Pausing stops this publisher on ALL campaigns on Retreaver.');
  });

  it('bell count', () => {
    assert.equal(bellCountLabel(0), '');
    assert.equal(bellCountLabel(7), '7');
    assert.equal(bellCountLabel(150), '99+');
  });
});

describe('alert row controls', () => {
  it('suggested Retreaver alert → Pause button', () => {
    assert.equal(alertButton({ scope: 'publisher', platform: 'retreaver', action: 'suggested' }), 'pause');
  });
  it('Ringba while unverified → link, never a Pause button', () => {
    assert.equal(
      alertButton({ scope: 'publisher', platform: 'ringba', action: 'suggested', ringba_link: 'https://app.ringba.com/#/dashboard/publishers' }),
      'ringba_link',
    );
  });
  it('paused alert with an open pause → Unpause', () => {
    assert.equal(alertButton({ scope: 'publisher', platform: 'retreaver', action: 'paused', pause_id: 'p1' }), 'unpause');
  });
  it('notify-only and campaign alerts have no control', () => {
    assert.equal(alertButton({ scope: 'publisher', platform: 'retreaver', action: 'none' }), null);
    assert.equal(alertButton({ scope: 'campaign', platform: 'retreaver', action: 'suggested' }), null);
  });
  it('Pings link (no publisher filter exists on the Pings page)', () => {
    assert.equal(pingsLinkFor({ entity_id: 'ID30', platform: 'retreaver' }), '/pings');
  });
});

describe('route visibility for the alert centre', () => {
  it('/alerts belongs to workflows', () => {
    assert.equal(featureForRoute('/alerts'), 'workflows');
    assert.equal(featureForRoute('/alertsx'), null);
  });
  it('hidden by default, visible with the workflows flag only', () => {
    assert.equal(isRouteVisible('/alerts', true, {}), false);
    assert.equal(isRouteVisible('/alerts', true, { workflows: 'true' }), true);
    assert.equal(isRouteVisible('/alerts', true, { ask_ai: 'true' }), false);
    assert.equal(isRouteVisible('/ask', true, { workflows: 'true' }), false);
  });
});

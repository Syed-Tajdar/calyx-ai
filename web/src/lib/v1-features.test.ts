import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  featureForRoute,
  isFeatureEnabled,
  isRouteVisible,
  NON_V1_ROUTES,
  platformsForV1,
} from './v1-features.ts';

describe('platformsForV1', () => {
  const all = ['ringba', 'retreaver', 'trackdrive', 'callgrid', 'callloom-native'];

  it('keeps Ringba and Retreaver when v1 is on', () => {
    assert.deepEqual(platformsForV1(all, true), ['ringba', 'retreaver']);
  });

  it('keeps every platform when v1 is off', () => {
    assert.deepEqual(platformsForV1(all, false), all);
  });
});

describe('route gating', () => {
  const v1Routes = ['/dashboard', '/pings', '/calls/abc', '/reports', '/coverage', '/settings'];
  const closed = [
    '/scoring',
    '/scoring/scorecards',
    '/intelligence',
    '/ask',
    '/alerts',
    '/compliance',
    '/attribution',
  ];

  it('maps each out-of-scope route to its feature', () => {
    assert.equal(featureForRoute('/scoring'), 'qa');
    assert.equal(featureForRoute('/scoring/scorecards'), 'qa');
    assert.equal(featureForRoute('/ask'), 'ask_ai');
    assert.equal(featureForRoute('/ask?q=x'), 'ask_ai');
    assert.equal(featureForRoute('/intelligence'), 'intelligence');
    assert.equal(featureForRoute('/compliance'), 'compliance');
    assert.equal(featureForRoute('/attribution'), 'attribution');
    for (const r of v1Routes) assert.equal(featureForRoute(r), null);
    // prefix match is per segment
    assert.equal(featureForRoute('/asking'), null);
    assert.equal(featureForRoute('/scoringx'), null);
  });

  it('lists exactly the six out-of-scope prefixes', () => {
    assert.deepEqual([...NON_V1_ROUTES].sort(), [
      '/alerts',
      '/ask',
      '/attribution',
      '/compliance',
      '/intelligence',
      '/scoring',
    ]);
  });

  it('closes every out-of-scope route when v1 is on, keeps v1 routes', () => {
    for (const r of closed) assert.equal(isRouteVisible(r, true, {}), false, r);
    for (const r of v1Routes) assert.equal(isRouteVisible(r, true, {}), true, r);
  });

  it('opens everything when v1 is off', () => {
    for (const r of [...closed, ...v1Routes]) assert.equal(isRouteVisible(r, false, {}), true, r);
  });

  it('a v2 flag re-opens only its own routes', () => {
    const askOnly = { ask_ai: 'true' };
    assert.equal(isRouteVisible('/ask', true, askOnly), true);
    for (const r of closed.filter((x) => x !== '/ask')) {
      assert.equal(isRouteVisible(r, true, askOnly), false, r);
    }
    const qaOnly = { qa: 'true' };
    assert.equal(isRouteVisible('/scoring', true, qaOnly), true);
    assert.equal(isRouteVisible('/scoring/scorecards', true, qaOnly), true);
    assert.equal(isRouteVisible('/ask', true, qaOnly), false);
  });

  it('only an exact "true" counts as a flag', () => {
    assert.equal(isFeatureEnabled('ask_ai', true, { ask_ai: '1' }), false);
    assert.equal(isFeatureEnabled('ask_ai', true, { ask_ai: undefined }), false);
  });
});

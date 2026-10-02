import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { platformsForV1 } from './v1-features.ts';

describe('platformsForV1', () => {
  const all = ['ringba', 'retreaver', 'trackdrive', 'callgrid', 'callloom-native'];

  it('keeps Ringba and Retreaver when v1 is on', () => {
    assert.deepEqual(platformsForV1(all, true), ['ringba', 'retreaver']);
  });

  it('keeps every platform when v1 is off', () => {
    assert.deepEqual(platformsForV1(all, false), all);
  });
});

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { canSeeWritebackActions } from './writeback-role.ts';

describe('canSeeWritebackActions', () => {
  it('shows the controls only to an admin', () => {
    assert.equal(canSeeWritebackActions('admin'), true);
    assert.equal(canSeeWritebackActions('viewer'), false);
    assert.equal(canSeeWritebackActions('member'), false);
    assert.equal(canSeeWritebackActions(null), false);
    assert.equal(canSeeWritebackActions(undefined), false);
  });
});

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { buyerLabel } from './buyer-label.ts';

describe('buyerLabel', () => {
  it('says "Buyer (target)" for Retreaver calls', () => {
    assert.equal(buyerLabel('retreaver'), 'Buyer (target)');
    assert.equal(buyerLabel('Retreaver'), 'Buyer (target)');
  });
  it('says "Buyer" for Ringba and anything else', () => {
    assert.equal(buyerLabel('ringba'), 'Buyer');
    assert.equal(buyerLabel(undefined), 'Buyer');
  });
});

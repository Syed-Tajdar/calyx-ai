import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { isConfirmableConvertAmount } from './writeback-amount.ts';

describe('isConfirmableConvertAmount', () => {
  it('allows confirming more than 25', () => {
    assert.equal(isConfirmableConvertAmount(26), true);
    assert.equal(isConfirmableConvertAmount(57), true);
    assert.equal(isConfirmableConvertAmount(1000), true);
  });

  it('rejects empty, zero, and negative', () => {
    assert.equal(isConfirmableConvertAmount(null), false);
    assert.equal(isConfirmableConvertAmount(0), false);
    assert.equal(isConfirmableConvertAmount(-1), false);
    assert.equal(isConfirmableConvertAmount(Number.NaN), false);
  });
});

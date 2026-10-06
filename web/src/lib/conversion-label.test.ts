import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { conversionStateLabel } from './conversion-label.ts';

describe('conversionStateLabel', () => {
  it('reads the stamp and does not invent a fifth state', () => {
    assert.equal(conversionStateLabel('converted'), 'Converted');
    assert.equal(conversionStateLabel('converted_zero'), 'Converted · $0');
    assert.equal(conversionStateLabel('not_converted'), 'Not converted');
    assert.equal(conversionStateLabel('not_reported'), 'n/a');
    assert.equal(conversionStateLabel(undefined), 'n/a');
  });
});

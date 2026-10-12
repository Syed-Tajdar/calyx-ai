import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { isConfirmedOutcome, writebackToast } from './writeback-toast.ts';

describe('writebackToast', () => {
  it('a confirmed write says confirmed (the bug: it said "had not confirmed yet")', () => {
    const t = writebackToast({ verb: 'Converted', platformLabel: 'Ringba', outcome: 'confirmed', conversionAmount: 18.4 });
    assert.equal(t.confirmed, true);
    assert.equal(t.title, 'Converted in Ringba');
    assert.equal(t.description, 'Confirmed by Ringba — now $18.40');
    assert.doesNotMatch(t.description, /not confirmed/);
  });

  it('a confirmed Retreaver void shows the new amount', () => {
    const t = writebackToast({ verb: 'Voided', platformLabel: 'Retreaver', outcome: 'confirmed', conversionAmount: 0 });
    assert.equal(t.title, 'Voided in Retreaver');
    assert.equal(t.description, 'Confirmed by Retreaver — now $0.00');
  });

  it('pending keeps the "not confirmed yet" wording (Ringba text unchanged)', () => {
    const t = writebackToast({ verb: 'Cancelled', platformLabel: 'Ringba', outcome: 'pending', conversionAmount: 5 });
    assert.equal(t.confirmed, false);
    assert.equal(t.title, 'Cancelled in Ringba');
    assert.equal(t.description, 'Sent. Ringba had not confirmed yet (pending) — it may take a moment.');
  });

  it('accepts the legacy "settled" value and guards a missing amount', () => {
    assert.equal(isConfirmedOutcome('settled'), true);
    assert.equal(isConfirmedOutcome('pending'), false);
    assert.equal(isConfirmedOutcome(undefined), false);
    const t = writebackToast({ verb: 'Converted', platformLabel: 'Ringba', outcome: 'confirmed', conversionAmount: undefined });
    assert.equal(t.description, 'Confirmed by Ringba — now $0.00');
  });
});

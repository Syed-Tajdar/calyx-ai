import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  canRequestTranscription,
  filterUtterances,
  formatBytes,
  formatOffset,
  formatUsd,
  speakerLabel,
  stateLabel,
} from './transcript.ts';

const us = [
  { idx: 0, speaker: 0, role: 'unknown', start_ms: 100, end_ms: 2400, text: 'Thank you for calling' },
  { idx: 1, speaker: 1, role: 'unknown', start_ms: 65_000, end_ms: 70_000, text: 'My card is [REDACTED]' },
  { idx: 2, speaker: 1, role: 'caller', start_ms: 3_723_000, end_ms: 3_725_000, text: 'Yes, Medicare' },
];

describe('transcript helpers', () => {
  it('formats offsets', () => {
    assert.equal(formatOffset(100), '0:00');
    assert.equal(formatOffset(65_000), '1:05');
    assert.equal(formatOffset(3_723_000), '1:02:03');
    assert.equal(formatOffset(-5), '0:00');
  });

  it('labels speakers: roles once known, otherwise the diarization number', () => {
    assert.equal(speakerLabel(us[0]), 'Speaker 1');
    assert.equal(speakerLabel(us[2]), 'Caller');
    assert.equal(speakerLabel({ role: 'agent', speaker: 0 }), 'Agent');
    assert.equal(speakerLabel({ role: 'unknown', speaker: null }), 'Speaker');
  });

  it('searches text and speaker, case-insensitive', () => {
    assert.equal(filterUtterances(us, '').length, 3);
    assert.deepEqual(filterUtterances(us, 'medicare').map((u) => u.idx), [2]);
    assert.deepEqual(filterUtterances(us, 'speaker 2').map((u) => u.idx), [1]);
    assert.deepEqual(filterUtterances(us, 'nothing here'), []);
  });

  it('says what state the transcript is in', () => {
    assert.equal(stateLabel('done'), 'Ready');
    assert.match(stateLabel('queued'), /^Pending/);
    assert.match(stateLabel('capped'), /cap/);
    assert.match(stateLabel('no_recording', 'not_answered'), /not answered/);
    assert.match(stateLabel('no_recording', 'no_access'), /Recording Access/);
    assert.equal(stateLabel('none'), 'Not transcribed');
  });

  it('offers Transcribe only when nothing is pending or done', () => {
    for (const s of ['none', 'failed', 'no_recording']) assert.equal(canRequestTranscription(s), true);
    for (const s of ['queued', 'running', 'capped', 'done']) assert.equal(canRequestTranscription(s), false);
  });

  it('formats sizes and dollars', () => {
    assert.equal(formatBytes(0), '0 B');
    assert.equal(formatBytes(598_144), '598 KB');
    assert.equal(formatBytes(11_200_000_000), '11.2 GB');
    assert.equal(formatUsd(1.5), '$1.50');
  });
});

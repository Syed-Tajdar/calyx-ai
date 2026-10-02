import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  chartSeriesForPreset,
  grainForSpan,
  periodBadge,
  rollupDaily,
} from './ping-series.ts';

describe('ping series period', () => {
  it('names the selected period', () => {
    assert.equal(periodBadge('7d'), '7 days');
    assert.equal(periodBadge('30d'), '30 days');
    assert.equal(periodBadge('all'), 'All time');
  });

  it('keeps all-time under about 60 points and preserves the total', () => {
    const daily = Array.from({ length: 400 }, (_, i) => ({
      date: new Date(Date.UTC(2025, 0, 1 + i)).toISOString().slice(0, 10),
      count: 1,
    }));
    assert.equal(grainForSpan(400), 'week');
    const rolled = chartSeriesForPreset(daily, 'all');
    assert.ok(rolled.length <= 60);
    assert.equal(
      rolled.reduce((sum, point) => sum + point.count, 0),
      400,
    );
    assert.equal(chartSeriesForPreset(daily.slice(0, 30), '30d').length, 30);
    assert.equal(rollupDaily(daily.slice(0, 7), 'day').length, 7);
  });
});

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  axisDateTicks,
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

  it('fills every day through today and keeps the call total', () => {
    const today = new Date(2026, 9, 8, 15, 0, 0);
    const series = chartSeriesForPreset([{ date: '2026-10-06', count: 4 }], '7d', today);
    assert.equal(series.length, 7);
    assert.equal(series[0].date, '2026-10-02');
    assert.equal(series[series.length - 1].date, '2026-10-08');
    assert.equal(series.find((point) => point.date === '2026-10-05')?.count, 0);
    assert.equal(series.reduce((sum, point) => sum + point.count, 0), 4);

    const month = chartSeriesForPreset([{ date: '2026-09-22', count: 9 }], '30d', today);
    assert.equal(month.length, 30);
    assert.equal(month[0].date, '2026-09-09');
    assert.equal(month[month.length - 1].date, '2026-10-08');
    assert.equal(month.reduce((sum, point) => sum + point.count, 0), 9);

    const all = chartSeriesForPreset([{ date: '2026-09-22', count: 9 }], 'all', today);
    assert.equal(all[all.length - 1].date, '2026-10-08');
    assert.equal(all.reduce((sum, point) => sum + point.count, 0), 9);
  });

  it('shows at most 8 x-axis labels for 30 daily points', () => {
    const dates = Array.from({ length: 30 }, (_, i) =>
      new Date(Date.UTC(2026, 8, 9 + i)).toISOString().slice(0, 10),
    );
    const ticks = axisDateTicks(dates);
    assert.equal(ticks[0], dates[0]);
    assert.equal(ticks[ticks.length - 1], dates[dates.length - 1]);
    assert.ok(ticks.length <= 8);
    assert.equal(axisDateTicks(dates.slice(0, 7)).length, 7);
  });
});

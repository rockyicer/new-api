import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildRecentDayLabels,
  aggregateQuotaDataByDayAndModel,
  buildDailyQuotaBarSeries,
  buildDailyQuotaLineSeries,
} from './dashboard-chart-data.js';

test('buildRecentDayLabels returns 14 continuous day labels', () => {
  const endTimestamp = Date.UTC(2026, 2, 20, 12, 0, 0) / 1000;
  const labels = buildRecentDayLabels(endTimestamp, 14);

  assert.equal(labels.length, 14);
  assert.deepEqual(labels.slice(0, 3), ['03-07', '03-08', '03-09']);
  assert.deepEqual(labels.slice(-3), ['03-18', '03-19', '03-20']);
});

test('aggregateQuotaDataByDayAndModel folds hourly data into daily model buckets and fills zeros', () => {
  const endTimestamp = Date.UTC(2026, 2, 20, 12, 0, 0) / 1000;
  const data = [
    {
      model_name: 'gpt-4o-mini',
      quota: 100,
      created_at: Date.UTC(2026, 2, 19, 9, 0, 0) / 1000,
    },
    {
      model_name: 'gpt-4o-mini',
      quota: 50,
      created_at: Date.UTC(2026, 2, 19, 18, 0, 0) / 1000,
    },
    {
      model_name: 'claude-3-5-haiku',
      quota: 80,
      created_at: Date.UTC(2026, 2, 20, 8, 0, 0) / 1000,
    },
  ];

  const aggregated = aggregateQuotaDataByDayAndModel(data, endTimestamp, 14);
  const march19Mini = aggregated.find(
    (item) => item.Time === '03-19' && item.Model === 'gpt-4o-mini',
  );
  const march20Claude = aggregated.find(
    (item) => item.Time === '03-20' && item.Model === 'claude-3-5-haiku',
  );
  const march18Mini = aggregated.find(
    (item) => item.Time === '03-18' && item.Model === 'gpt-4o-mini',
  );

  assert.equal(march19Mini.rawQuota, 150);
  assert.equal(march20Claude.rawQuota, 80);
  assert.equal(march18Mini.rawQuota, 0);
  assert.equal(aggregated.filter((item) => item.Time === '03-20').length, 2);
});

test('buildDailyQuotaBarSeries keeps stacked quota totals by day', () => {
  const aggregated = [
    { Time: '03-19', Model: 'gpt-4o-mini', rawQuota: 150 },
    { Time: '03-19', Model: 'claude-3-5-haiku', rawQuota: 20 },
    { Time: '03-20', Model: 'gpt-4o-mini', rawQuota: 0 },
    { Time: '03-20', Model: 'claude-3-5-haiku', rawQuota: 80 },
  ];

  const barSeries = buildDailyQuotaBarSeries(aggregated);
  const march19Sum = barSeries
    .filter((item) => item.Time === '03-19')
    .reduce((sum, item) => sum + item.rawQuota, 0);

  assert.equal(march19Sum, 170);
  assert.equal(
    barSeries.find(
      (item) => item.Time === '03-20' && item.Model === 'gpt-4o-mini',
    ).Usage,
    0,
  );
});

test('buildDailyQuotaLineSeries uses quota instead of count for trend values', () => {
  const aggregated = [
    { Time: '03-19', Model: 'gpt-4o-mini', rawQuota: 150 },
    { Time: '03-20', Model: 'gpt-4o-mini', rawQuota: 60 },
  ];

  const lineSeries = buildDailyQuotaLineSeries(aggregated);

  assert.deepEqual(lineSeries, [
    { Time: '03-19', Model: 'gpt-4o-mini', Quota: 150 },
    { Time: '03-20', Model: 'gpt-4o-mini', Quota: 60 },
  ]);
});

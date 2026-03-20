const DAY_IN_SECONDS = 24 * 60 * 60;

const pad2 = (value) => String(value).padStart(2, '0');

const toLocalDayStart = (timestamp) => {
  const date = new Date(timestamp * 1000);
  date.setHours(0, 0, 0, 0);
  return date;
};

const formatDayLabel = (date, showYear) => {
  const year = date.getFullYear();
  const month = pad2(date.getMonth() + 1);
  const day = pad2(date.getDate());
  return showYear ? `${year}-${month}-${day}` : `${month}-${day}`;
};

const shouldShowYear = (endTimestamp, days) => {
  const endDate = toLocalDayStart(endTimestamp);
  const startDate = new Date(endDate);
  startDate.setDate(startDate.getDate() - (days - 1));
  return startDate.getFullYear() !== endDate.getFullYear();
};

export const buildRecentDayLabels = (endTimestamp, days = 14) => {
  const labels = [];
  const endDate = toLocalDayStart(endTimestamp);
  const showYear = shouldShowYear(endTimestamp, days);

  for (let index = days - 1; index >= 0; index--) {
    const current = new Date(endDate);
    current.setDate(endDate.getDate() - index);
    labels.push(formatDayLabel(current, showYear));
  }

  return labels;
};

export const buildRecentDayWindow = (endTimestamp, days = 14) => {
  const endDate = toLocalDayStart(endTimestamp);
  const startDate = new Date(endDate);
  startDate.setDate(endDate.getDate() - (days - 1));

  return {
    startTimestamp: Math.floor(startDate.getTime() / 1000),
    endTimestamp,
  };
};

export const aggregateQuotaDataByDayAndModel = (
  data,
  endTimestamp,
  days = 14,
) => {
  const labels = buildRecentDayLabels(endTimestamp, days);
  const labelSet = new Set(labels);
  const showYear = shouldShowYear(endTimestamp, days);
  const models = Array.from(
    new Set(
      (data || [])
        .map((item) => item?.model_name)
        .filter((modelName) => typeof modelName === 'string' && modelName),
    ),
  );

  if (models.length === 0) {
    models.push('无数据');
  }

  const quotaMap = new Map();

  for (const item of data || []) {
    const label = formatDayLabel(toLocalDayStart(item.created_at), showYear);
    if (!labelSet.has(label)) {
      continue;
    }
    const modelName = item?.model_name || '无数据';
    const key = `${label}::${modelName}`;
    quotaMap.set(key, (quotaMap.get(key) || 0) + Number(item?.quota || 0));
  }

  return labels.flatMap((label) =>
    models.map((modelName) => ({
      Time: label,
      Model: modelName,
      rawQuota: quotaMap.get(`${label}::${modelName}`) || 0,
    })),
  );
};

export const buildDailyQuotaBarSeries = (aggregatedData) => {
  const totals = new Map();
  for (const item of aggregatedData) {
    totals.set(item.Time, (totals.get(item.Time) || 0) + item.rawQuota);
  }

  return aggregatedData.map((item) => ({
    ...item,
    Usage: item.rawQuota,
    TimeSum: totals.get(item.Time) || 0,
  }));
};

export const buildDailyQuotaLineSeries = (aggregatedData) =>
  aggregatedData.map((item) => ({
    Time: item.Time,
    Model: item.Model,
    Quota: item.rawQuota,
  }));

export const DAY_WINDOW_SECONDS = DAY_IN_SECONDS;

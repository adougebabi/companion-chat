export function validTimeRange(start, end) {
  const from = typeof start === 'string' ? Date.parse(start) : NaN;
  const to = typeof end === 'string' ? Date.parse(end) : NaN;
  return Number.isFinite(from) && Number.isFinite(to) && to > from;
}

export function isUnplannedActivity(value) {
  return typeof value === 'string' && ['默认未安排', '未安排时间'].includes(value.trim());
}

export function currentActivityDisplay(value) {
  return isUnplannedActivity(value) ? '此刻没有特别的安排' : value;
}

export function scheduleItemDisplay(item) {
  const source = item._source;
  const activity = typeof source?.activity === 'string' ? source.activity.trim() : '';
  const unplanned = isUnplannedActivity(activity || item.text)
    || (!activity && item._schedule?.reschedule_policy?.default_initial_schedule === true);
  const showTime = !unplanned && validTimeRange(source?.start_at, source?.end_at);
  return {
    unplanned,
    title: unplanned ? '这一天还没有安排' : item.text,
    showTime,
    timeNote: unplanned || showTime ? '' : '时间待核对',
  };
}

export function validTimeRange(start, end) {
  const from = typeof start === 'string' ? Date.parse(start) : NaN;
  const to = typeof end === 'string' ? Date.parse(end) : NaN;
  return Number.isFinite(from) && Number.isFinite(to) && to > from;
}

export function scheduleItemDisplay(item) {
  const source = item._source;
  const activity = typeof source?.activity === 'string' ? source.activity.trim() : '';
  const unplanned = ['默认未安排', '未安排时间'].includes(activity || item.text)
    || (!activity && item._schedule?.reschedule_policy?.default_initial_schedule === true);
  const showTime = !unplanned && validTimeRange(source?.start_at, source?.end_at);
  return {
    unplanned,
    title: unplanned ? '这一天还没有安排' : item.text,
    showTime,
    timeNote: unplanned || showTime ? '' : '时间待核对',
  };
}

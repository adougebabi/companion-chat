// Deterministic fixture for the 「约定与日程设置」 sheet. All instants are fixed
// and the browser clock is frozen by the test runner, so plan windows and
// labels render identically on every run. No network is involved.
import * as services from '@/v3/proto/services.js';

export const FIXED_NOW = '2026-11-16T10:30:00+08:00';
const TODAY = '2026-11-16';
const zone = 'Asia/Shanghai';

type Variant = 'default' | 'long' | 'empty';

const longText = '在旧街尽头那家只在周末营业、需要提前两天预约座位并且会根据天气调整菜单的小咖啡馆里，一边整理这周拍下的照片一边写下明天想去的地方';

function items(day: string, variant: Variant) {
  if (variant === 'empty') return [];
  const at = (h: string) => `${day}T${h}:00+08:00`;
  return [
    { id: 'item-reading', activity: variant === 'long' ? longText : '在窗边阅读', location: variant === 'long' ? '旧街咖啡馆（靠窗第二个位置，带插座，下午阳光会直射桌面）' : '家里书房', start_at: at('09:00'), end_at: at('10:00'), status: 'accepted', action_status: 'completed' },
    { id: 'item-walk', activity: '去花店买新的绣球', location: '街角花店', start_at: at('10:00'), end_at: at('11:30'), status: 'accepted', action_status: 'in_progress' },
    { id: 'item-call', activity: '和朋友视频通话', location: '家里', start_at: at('20:00'), end_at: at('21:00'), status: 'accepted', action_status: 'pending' },
  ];
}

export type FixtureEvent = { action: string; args: unknown[] };
export const events: FixtureEvent[] = [];
const record = (action: string, args: unknown[]) => { events.push({ action, args: JSON.parse(JSON.stringify(args ?? null)) }); };

export function installScheduleFixture(variant: Variant) {
  const detail = {
    context: { local_date: TODAY, timezone: zone },
    schedule: variant === 'empty' ? null : { local_date: TODAY, timezone: zone, items: items(TODAY, variant) },
    intentions: variant === 'empty' ? [] : [
      { action_intent: variant === 'long' ? longText : '周末去海边看日落', status: 'pending', confidence: 0.7 },
      { action_intent: '学会做一道新的汤', status: 'in_progress', confidence: 0.45 },
    ],
    goals: [],
    core_persona: { personality_system: { profiles: [{ id: 'p1', name: '日常', habits: variant === 'empty' ? '' : '早上喜欢先喝温水再看书；下午容易犯困，所以常把散步安排在午后；晚上不喜欢被打扰太久。' }] } },
    personality_runtime: { active_profile_id: 'p1' },
  };
  const promiseRows = variant === 'empty' ? [] : [
    { request_id: 'promise-1', title: variant === 'long' ? longText : '周六一起去看展', lifecycle: 'accepted', window_end: '2026-11-21T18:00:00+08:00', timezone: zone },
    { request_id: 'promise-2', title: '每天睡前道晚安', lifecycle: 'pending', window_end: '2026-11-30T23:00:00+08:00', timezone: zone },
  ];
  services.configureDomainService('life', {
    detail: async (...args: unknown[]) => { record('detail', args); return detail; },
    schedules: async (fid: string, query: any = {}) => {
      record('schedules', [fid, query]);
      const from = query.from || TODAY, to = query.to || from;
      const days: any[] = [];
      for (let d = new Date(from + 'T12:00:00Z'); d.toISOString().slice(0, 10) <= to; d.setUTCDate(d.getUTCDate() + 1)) {
        const local = d.toISOString().slice(0, 10);
        days.push({ local_date: local, schedule: variant === 'empty' ? null : { local_date: local, timezone: zone, items: items(local, variant) } });
      }
      return { timezone: zone, today: TODAY, from, to, days };
    },
    requests: async (...args: unknown[]) => { record('requests', args); return { items: [], next_cursor: '' }; },
    promises: async (...args: unknown[]) => { record('promises', args); return { items: promiseRows, next_cursor: '' }; },
    readPromise: async (_fid: string, id: string) => { record('readPromise', [id]); const p = promiseRows.find(row => row.request_id === id); return { ...p, window_start: '2026-11-21T10:00:00+08:00', scheduled_items: [] }; },
    promiseHistory: async () => ({ items: [], next_cursor: '' }),
    pendingPromise: () => null,
    schedulePending: () => null,
    scheduleItemPending: () => null,
  });
}

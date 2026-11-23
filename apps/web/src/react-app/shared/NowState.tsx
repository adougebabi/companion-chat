import JsonDetails from './JsonDetails';
import {currentActivityDisplay} from './schedule-presentation.mjs';

export const nowEmptyText = '此刻还没有可显示的状态';
const technicalValue = /^(?:(?:res|world_item|world|place|req)_[a-z\d_-]+(?:\s*·\s*v\d+(?:\.\d+)*)?|(?:fluctlight\.[a-z\d.-]+\.)?v\d+(?:\.\d+)*)$/i;

// Only replace standalone technical values. Leave descriptive text intact.
export function nowStateText(value: unknown, label: string) {
  if (typeof value !== 'string') return '';
  const text = value.trim();
  return technicalValue.test(text) ? `${label}详情待确认` : text;
}

export function nowStateSummary(character: {activity?: unknown; location?: unknown; mood?: unknown}) {
  return {
    activity: currentActivityDisplay(nowStateText(character.activity, '活动')),
    location: nowStateText(character.location, '地点'),
    mood: nowStateText(character.mood, '心情'),
  };
}

export function NowStateDownload({detail, world, schedule}: {
  detail?: {context?: unknown; inner_state?: unknown; current_state?: unknown};
  world?: unknown;
  schedule?: unknown;
}) {
  const {context, inner_state, current_state} = detail ?? {};
  if (context == null && inner_state == null && current_state == null && world == null && schedule == null) return null;
  return <JsonDetails value={{context, inner_state, current_state, world, schedule}} title="原始此刻记录（诊断）" downloadOnly/>;
}

import { esc, badge } from './controls.js';
import { Field, Button, KeyValueList } from './forms.js';

export function RoutineEditor({ slots, places }) {
  const options = [{ value: '', label: '选择地点' }, ...places.map(place => ({ value: place.id, label: place.name }))];
  return `<div class="stack" data-routine>${slots.map((slot, index) => `<div class="card" data-slot="${index}"><div class="grid g3">${Field({ label: '适用日', name: `day_${index}`, value: slot.day, type: 'select', options: ['每天', '工作日', '周末'] })}${Field({ label: '开始', name: `start_${index}`, value: slot.start, type: 'time' })}${Field({ label: '结束', name: `end_${index}`, value: slot.end, type: 'time' })}</div><div class="row" style="margin-top:8px">${Field({ label: '地点', name: `place_${index}`, value: slot.placeId, type: 'select', options })}${Field({ label: '活动', name: `activity_${index}`, value: slot.activity })}${Button({ label: '移除时段', tone: 'ghost danger', size: 'sm', attrs: { 'data-remove-slot': index } })}</div></div>`).join('')}</div>${Button({ label: '添加规律时段', attrs: { 'data-add-slot': true } })}`;
}
export function PositionSummary({ npc, placeName }) {
  const mode = npc.positionMode || '默认地点';
  return `${KeyValueList([['位置模式', esc(mode)], ['初始 / 默认地点', esc(placeName(npc.placeId))], ['生活规律', npc.routine?.length ? `${npc.routine.length} 个时段 ${badge('计划')}` : '尚未设置'], ['运行推进',npc.runtime_clock?.error_code?`上次失败 ${esc(npc.runtime_clock.error_code)} · 下次重试 ${esc(npc.runtime_clock.next_attempt_at)}`:npc.runtime_clock?'已进入后台推进':'尚未执行'],['实际运行在场',npc.current_presence?`${esc(placeName(npc.current_presence.place_id))} · ${esc(npc.current_presence.start_at)} — ${esc(npc.current_presence.end_at)} ${badge('运行记录')}`:'尚无有效执行记录'], ['上次确认位置', npc.confirmedPlaceId ? `${esc(placeName(npc.confirmedPlaceId))} · ${esc(npc.confirmedAt)}` : '无确认记录']], { className: 'kv small' })}<p class="small muted">规律和默认地点仍是预测；实际在场需要执行记录。上次接触地点是历史，不能替代当前在场。浏览此页不移动角色，也不创建相遇。</p>`;
}

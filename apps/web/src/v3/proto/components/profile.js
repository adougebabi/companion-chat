import { esc, badge } from './controls.js';
import { Card, EntityHeader } from './cards.js';
import { Field, KeyValueList } from './forms.js';
import { JsonDetails } from './records.js';
import { RevisionHistory } from './revisions.js';

const labels = { name: '姓名', age: '年龄', gender: '性别', occupation: '职业', background_story: '背景经历', background: '背景', timezone: '时区', location: '地点', nationality: '国籍', birthday: '生日', description: '说明', traits: '性格特征', interests: '兴趣', preferences: '偏好', values: '价值观', speech_style: '表达方式', revision: '版本', active_profile_id: '当前人格', status: '状态', mode: '人格模式', current_task: '当前活动', mood: '心情', energy: '精力', appearance: '有效外观', worn_items: '当前穿着', used_items: '当前使用', source: '来源', reason: '原因', claim: '自我认知', confidence: '置信度', updated_at: '更新时间', last_switch_at: '上次切换', source_revision: '来源版本', overlay_revision: '叠加版本' };
export function ProfileValue(value) {
  if (value === undefined) return '<span class="small muted">尚未提供</span>';
  if (value === null) return '<span class="small muted">未记录 / 已清除</span>';
  if (typeof value === 'object') return JsonDetails(value);
  if (value === '') return '<span class="small muted">空值</span>';
  return esc(typeof value === 'boolean' ? (value ? '是' : '否') : value);
}
function objectRows(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return ProfileValue(value);
  const entries = Object.entries(value);
  return entries.length ? KeyValueList(entries.map(([key, item]) => [labels[key] || key, ProfileValue(item)])) : '<p class="small muted">暂无记录</p>';
}
export function ProfileGroups(snapshot) {
  return `<div class="grid g2">${snapshot.groups.map(group => Card({ title: group.title, body: objectRows(group.value), attrs: { 'data-profile-group': group.key } })).join('')}</div>`;
}
export function ProfileHeader(snapshot, canEdit) {
  return Card({ attrs: { 'data-test': `profile-${snapshot.id}` },
    heading: snapshot.avatar ? EntityHeader({ avatar: snapshot.avatar, title: snapshot.name || '姓名尚未记录', subtitle: snapshot.mode === 'mock' ? '隔离演示资料' : '角色资料', status: badge(snapshot.status || '状态未提供') }) : `<div class="row"><h2 class="grow">${esc(snapshot.name || '姓名尚未记录')}</h2>${badge(snapshot.status || '状态未提供')}</div>`,
    body: `<p data-test="profile-bio">${esc(snapshot.bio) || '尚未记录简介'}</p>${KeyValueList([['身份核心版本', ProfileValue(snapshot.foundationRevision)], ['运行版本', ProfileValue(snapshot.runtimeRevision)], ['当前人格', ProfileValue(snapshot.profiles.find(p => p.id === snapshot.activeProfileId)?.name ?? snapshot.activeProfileId)]])}`,
    footer: `<div class="row"><button class="btn" data-profile-edit data-test="profile-edit" ${!canEdit || snapshot.readOnly ? 'disabled' : ''}>编辑简介…</button><a class="btn ghost" href="#/c/${encodeURIComponent(snapshot.id)}/settings/autonomy">自治与触达</a><a class="btn ghost" href="#/c/${encodeURIComponent(snapshot.id)}/growth/self">自我认知</a></div>` });
}
export function PersonaSection(snapshot, canSwitch) {
  const choices = snapshot.profiles.filter(profile => profile.id !== snapshot.activeProfileId && (!snapshot.switchTargetIds || snapshot.switchTargetIds.includes(profile.id)));
  const enabled = canSwitch && !snapshot.readOnly && snapshot.activeProfileId !== null && snapshot.foundationRevision !== null && snapshot.runtimeRevision !== null && choices.length;
  return Card({ title: '人格档案与切换', attrs: { 'data-test': 'profile-personas' }, body: `
    <p class="small muted">人格切换由受控命令执行。生活状态、有效外观和已有经历继续保留。</p>
    ${snapshot.profiles.map(profile => `<details class="mgmt"><summary>${esc(profile.name || profile.id)} ${profile.id === snapshot.activeProfileId ? badge('当前人格') : badge('人格档案', 'plain')}</summary>${objectRows(profile)}</details>`).join('') || '<p class="small muted">尚未提供人格档案</p>'}
    ${!canSwitch ? '<p class="small muted" data-test="profile-switch-unavailable">人格切换尚未连接受控命令，当前不能提交。</p>' : ''}
    <form class="stack" data-profile-switch novalidate>
    ${Field({ label: '目标人格', name: 'targetPersonaId', type: 'select', value: '', options: [{ value: '', label: '请选择目标人格' }, ...choices.map(p => ({ value: p.id, label: String(p.name || p.id) }))], attrs: { disabled: !enabled, required: true } })}
    ${Field({ label: '切换原因', name: 'reason', type: 'textarea', hint: '说明这次切换的原因，提交前需确认。', attrs: { disabled: !enabled, required: true, maxlength: 1000 } })}
    <label class="toggle"><input type="checkbox" name="confirm" ${!enabled ? 'disabled' : ''}> 我确认切换到所选人格</label>
    <div data-profile-feedback role="status" aria-live="polite"></div>
    <div class="row"><button class="btn accent" data-test="profile-switch-submit" ${!enabled ? 'disabled' : ''}>确认切换</button><button class="btn" type="button" data-profile-refresh>重新读取</button></div></form>` });
}
export function PersonaHistory(snapshot) {
  return `<div class="grid g2">${Card({ title: '人格切换历史', body: (snapshot.historyHasMore ? '<p class="small muted">显示最近 100 条切换记录，另有更早历史。</p>' : '') + (snapshot.history.length ? snapshot.history.map(entry => `<div class="lrow small"><span class="grow">${esc(entry.from_profile_id || '—')} → ${esc(entry.to_profile_id || entry.target_profile_id || '—')}<div class="muted">${esc(entry.reason || '')}</div>${JsonDetails(entry, '查看切换记录')}</span><time>${esc(entry.created_at || entry.occurred_at || '')}</time></div>`).join('') : '<p class="small muted">暂无切换记录</p>') })}${Card({ title: '身份核心修订', body: RevisionHistory({ entries: snapshot.revisions.map(r => ({ ...r, version: r.revision ?? r.version, at: r.created_at ?? r.at })), renderEntry: r => `${esc(r.reason || r.source || '')} ${badge(r.status || '版本记录')}${JsonDetails(r, '查看修订内容')}` }) })}</div>`;
}

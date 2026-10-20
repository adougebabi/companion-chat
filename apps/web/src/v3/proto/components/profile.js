import { esc, badge } from './controls.js';
import { Card, EntityHeader } from './cards.js';
import { Field, KeyValueList } from './forms.js';
import { JsonDetails } from './records.js';
import { RevisionHistory } from './revisions.js';
import {SemanticFields,SemanticValue,hasSemanticValue} from './semantic-fields.js';

export function ProfileValue(value) {
  if (value === undefined) return '<span class="small muted">尚未提供</span>';
  if (value === null) return '<span class="small muted">未记录 / 已清除</span>';
  if (typeof value === 'object') return JsonDetails(value);
  if (value === '') return '<span class="small muted">空值</span>';
  return esc(typeof value === 'boolean' ? (value ? '是' : '否') : value);
}
export function personaName(snapshot,id){return snapshot.profiles.find(p=>p.id===id)?.name||(id==='default'?'主人格':id?'未命名人格':'当前人格待读取');}
export function ProfileGroups(snapshot) {
  const group=key=>snapshot.groups.find(g=>g.key===key)?.value,core=group('foundation')||{},working=group('working')||{};
  const portrait=working.working_persona?.portrait_text;
  const sections=[];
  const add=(key,title,value,options={})=>{if(hasSemanticValue(value))sections.push([key,title,SemanticFields(value,options)]);};
  const identity=Object.fromEntries(Object.entries(group('identity')||{}).filter(([key])=>!['name','background','background_story'].includes(key)));
  add('identity','身份与背景',identity);add('personality','共同性格',group('personality'));add('policy','表达与相处',snapshot.groups.some(g=>g.key==='policy')?group('policy'):core.behavioral_policy);add('life','生活设定',snapshot.groups.some(g=>g.key==='life')?group('life'):core.life_profile);
  const current=group('current');if(current)sections.push(['current','此刻状态',SemanticFields(current)+(Object.hasOwn(current,'appearance')&&current.appearance===null?'<p class="small muted">有效外观：未记录 / 已清除。不会使用旧初始外观代替。</p>':'')]);
  const claims=group('developing')?.claims;
  sections.push(['developing','逐渐形成的自我认识',Array.isArray(claims)&&claims.length?claims.map(c=>`<div class="lrow"><div class="grow"><p>${esc(c.claim||c.text||'尚无认识内容')}</p>${SemanticValue(c.value)}<p class="small muted">${esc(({active:'当前认可',uncertain:'仍在探索',forgotten:'已遗忘',superseded:'已有新认识'})[c.status]||'状态待确认')}</p></div></div>`).join(''):'<p class="small muted">还没有形成已记录的自我认识，会随着真实经历逐步积累。</p>']);
  if(portrait)sections.push(['working','当前表达画像',`<p class="memory-content">${esc(portrait)}</p><p class="small muted">来自当前已准备的工作人格。</p>`]);
  const shared=core.personality_system||{};if([shared.core_relationship,shared.core_conflict,shared.integration].some(hasSemanticValue))sections.push(['shared','人格间的关系与协作',SemanticFields({core_relationship:shared.core_relationship,core_conflict:shared.core_conflict,integration:shared.integration})]);
  return `<div class="grid g2 profile-sections">${sections.map(([key,title,body])=>Card({title,body,attrs:{'data-profile-group':key}})).join('')}</div><details class="card profile-technical"><summary>完整设定与来源（技术详情）</summary><p class="small muted">原始字段、版本与来源保留在这里，供核对和排查。</p>${snapshot.groups.map(g=>JsonDetails(g.value,g.title)).join('')}</details>`;
}
export function ProfileHeader(snapshot, canEdit) {
  return Card({ attrs: { 'data-test': `profile-${snapshot.id}` },
    heading: snapshot.avatar ? EntityHeader({ avatar: snapshot.avatar, title: snapshot.name || '姓名尚未记录', subtitle: snapshot.mode === 'mock' ? '隔离演示资料' : '角色资料', status: badge(({active:'运行中',paused:'已暂停',retired:'已退休',archived:'已归档',created:'准备中',failed:'需要处理'})[snapshot.status]||snapshot.status||'状态未提供') }) : `<div class="row"><h2 class="grow">${esc(snapshot.name || '姓名尚未记录')}</h2>${badge(({active:'运行中',paused:'已暂停',retired:'已退休',archived:'已归档'})[snapshot.status]||snapshot.status||'状态未提供')}</div>`,
    body: `<p class="memory-content" data-test="profile-bio">${esc(snapshot.bio) || '尚未记录简介'}</p><p class="small muted">当前人格 · ${esc(personaName(snapshot,snapshot.activeProfileId))}</p><details class="mgmt small muted"><summary>资料版本</summary>${KeyValueList([['身份核心版本', ProfileValue(snapshot.foundationRevision)], ['运行版本', ProfileValue(snapshot.runtimeRevision)]])}</details>`,
    footer: `<div class="row"><button class="btn" data-profile-edit data-test="profile-edit" ${!canEdit || snapshot.readOnly ? 'disabled' : ''}>编辑简介…</button><a class="btn ghost" href="#/c/${encodeURIComponent(snapshot.id)}/settings/autonomy">自治与触达</a><a class="btn ghost" href="#/c/${encodeURIComponent(snapshot.id)}/growth/self">自我认知</a></div>` });
}
export function PersonaSection(snapshot, canSwitch) {
  const choices = snapshot.profiles.filter(profile => profile.id !== snapshot.activeProfileId && (!snapshot.switchTargetIds || snapshot.switchTargetIds.includes(profile.id)));
  const enabled = canSwitch && !snapshot.readOnly && snapshot.activeProfileId !== null && snapshot.foundationRevision !== null && snapshot.runtimeRevision !== null && choices.length;
  return Card({ title: '人格档案与切换', attrs: { 'data-test': 'profile-personas' }, body: `
    <p class="small muted">人格切换由受控命令执行。生活状态、有效外观和已有经历继续保留。</p>
    <div class="grid g2 profile-sections">${snapshot.profiles.map((profile,index)=>`<section class="persona-sheet"><div class="row"><h4 class="grow">${esc(profile.name||`人格档案 ${index+1}（未命名）`)}</h4>${profile.id===snapshot.activeProfileId?badge('当前人格'):badge('人格档案','plain')}</div>${SemanticFields(profile,{exclude:['name'],empty:'这个人格尚未补充具体设定。'})}${JsonDetails(profile,'此人格的完整设定记录')}</section>`).join('') || '<p class="small muted">尚未提供人格档案</p>'}</div>
    ${!canSwitch ? '<p class="small muted" data-test="profile-switch-unavailable">人格切换尚未连接受控命令，当前不能提交。</p>' : ''}
    <details class="persona-controls"><summary>切换当前人格</summary><form class="stack" data-profile-switch novalidate>
    ${Field({ label: '目标人格', name: 'targetPersonaId', type: 'select', value: '', options: [{ value: '', label: '请选择目标人格' }, ...choices.map(p => ({ value: p.id, label: personaName(snapshot,p.id) }))], attrs: { disabled: !enabled, required: true } })}
    ${Field({ label: '切换原因', name: 'reason', type: 'textarea', hint: '说明这次切换的原因，提交前需确认。', attrs: { disabled: !enabled, required: true, maxlength: 1000 } })}
    <label class="toggle"><input type="checkbox" name="confirm" ${!enabled ? 'disabled' : ''}> 我确认切换到所选人格</label>
    <div data-profile-feedback role="status" aria-live="polite"></div>
    <div class="row"><button class="btn accent" data-test="profile-switch-submit" ${!enabled ? 'disabled' : ''}>确认切换</button><button class="btn" type="button" data-profile-refresh>重新读取</button></div></form></details>` });
}
export function PersonaHistory(snapshot) {
  return `<details class="card"><summary>人格切换与资料修订历史</summary><div class="grid g2 profile-sections">${Card({ title: '人格切换历史', body: (snapshot.historyHasMore ? '<p class="small muted">显示最近 100 条切换记录，另有更早历史。</p>' : '') + (snapshot.history.length ? snapshot.history.map(entry => `<div class="lrow small"><span class="grow">${esc(personaName(snapshot,entry.from_profile_id))} → ${esc(personaName(snapshot,entry.to_profile_id||entry.target_profile_id))}<div class="muted">${esc(entry.reason || '')}</div>${JsonDetails(entry, '查看切换记录')}</span><time>${esc(entry.created_at || entry.occurred_at || '')}</time></div>`).join('') : '<p class="small muted">暂无切换记录</p>') })}${Card({ title: '身份核心修订', body: RevisionHistory({ entries: snapshot.revisions.map(r => ({ ...r, version: r.revision ?? r.version, at: r.created_at ?? r.at })), renderEntry: r => `${esc(r.reason || r.source || '')} ${badge(r.status || '版本记录')}${JsonDetails(r, '查看修订内容')}` }) })}</div></details>`;
}

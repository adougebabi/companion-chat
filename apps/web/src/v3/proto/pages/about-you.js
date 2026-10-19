import { providerPage } from './provider-page.js';
import * as store from '../store.js';
import { esc, icon, badge, toast, requireOwner, drawer, objLink } from '../ui.js';
import { Field, Tabs, Button } from '../components/forms.js';
import { RevisionHistory, ConflictPanel } from '../components/revisions.js';
import { mockClient } from '../services.js';
import { AboutYouCard } from '../components/about-you-card.js';

const sessions = new Map();
store.subscribe(event => { if (event.type === 'reset') sessions.clear(); });
const groups = {
  basic: ['称呼', '城市', '职业', '喜好', '可让她知道'],
  relationship: ['你们的关系（自述）', '双方距离与交流方式', '性格', '习惯', '希望她知道的事'],
  rules: ['给她的规则与边界'],
};
const multiline = new Set(['可让她知道', '希望她知道的事', '性格', '习惯', '给她的规则与边界']);

export function renderAboutYou(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-about-you.js'), 'renderRealAboutYou',el,ctx);
  let alive = true;
  const actorId = ctx.query.actor || store.get('user').id, characterId = ctx.charId;
  const key = `${characterId}:${actorId}`, tab = Object.hasOwn(groups, ctx.query.tab) ? ctx.query.tab : 'basic';
  el.innerHTML = '<div class="card"><div class="skel"></div><p class="small muted">正在读取 Mock 资料…</p></div>';
  async function paint() {
    const saved = await mockClient.aboutYou(characterId, actorId); if (!alive) return;
    if (!sessions.has(key)) sessions.set(key, { fields: { ...saved.fields }, base: saved.version, dirty: false, conflict: false });
    const session = sessions.get(key), character = store.get('characters', characterId);
    const tabs = [{ value: 'basic', label: '基本资料' }, { value: 'relationship', label: '关系与自述' }, { value: 'rules', label: '给她的规则' }].map(item => ({ ...item, href: `#/c/${characterId}/people/owner?actor=${actorId}&tab=${item.value}` }));
    const fields = groups[tab].map(name => Field({ label: name, name, value: session.fields[name] || '', type: tab === 'basic' ? 'text' : multiline.has(name) ? 'textarea' : 'text', rows: name === '给她的规则与边界' ? 6 : 3, attrs: { 'data-profile-field': name, 'data-test': `owner-${name}`, maxlength: 8000 } })).join('');
    el.innerHTML = `${Tabs({ items: tabs, value: tab })}<div class="row" style="margin-bottom:12px">${Field({ label: '交流参与者', name: 'actor', value: actorId, type: 'select', options: mockClient.actors().map(actor => ({ value: actor.id, label: actor.name })), attrs: { 'data-actor': true } })}<span class="small muted">${esc(character.name)}的交流范围 · ${badge('Mock', 'plain')}</span></div>
      ${AboutYouCard({subtitle:`${character.name}会把这些作为你填写的资料。只填你愿意让她知道的。`,fields,formAttrs:{'data-of':true},conflict:session.conflict ? ConflictPanel({ baseVersion: session.base, currentVersion: saved.version, rows: groups[tab].filter(name => session.fields[name] !== saved.fields[name]).map(name => ({ field: name, before: saved.fields[name] || '未填写', after: session.fields[name] || '未填写' })) }) : '',advanced:tab === 'basic' ? `<details class="mgmt small muted"><summary>更多基本资料</summary><div class="stack" style="margin-top:8px">${['姓名', '年龄', '性别'].map(name => Field({ label: name, name, value: session.fields[name] || '', attrs: { 'data-profile-field': name, maxlength: 1024 } })).join('')}</div></details>` : '',actions:`<button type="reset" class="btn" data-test="owner-cancel">撤销更改</button><button class="btn accent" data-test="owner-save">保存</button><span class="small muted">v${session.base}${session.dirty ? ' · 有未保存修改' : ''}</span>`,result:'<p class="small muted" data-result></p>'})}
      <details class="mgmt small muted" style="max-width:640px"><summary>来源与修订历史</summary><p>本人填写的资料与${esc(character.name)}对你的印象分开保存。</p>${saved.impression ? `<div class="card"><h3>她对你的印象（只读 Mock）</h3><p>${esc(saved.impression.text)}</p><span class="small muted">来源 ${objLink(saved.impression.source)}</span></div>` : '<p>这个范围尚无角色印象。</p>'}${RevisionHistory({ entries: saved.history, renderEntry: entry => `${badge('所有者修改')} ${esc(entry.operation === 'correct' ? '纠正资料' : '情况变化')} · ${esc(entry.source)}` })}${Button({ label: '模拟另一处修改（冲突演示）', tone: 'ghost', size: 'sm', attrs: { 'data-simulate-profile': true } })}</details>`;
    el.querySelector('[data-actor]').onchange = event => ctx.navigate(`#/c/${characterId}/people/owner?actor=${encodeURIComponent(event.target.value)}&tab=${tab}`);
    el.querySelectorAll('[data-profile-field]').forEach(input => input.oninput = () => { session.fields[input.dataset.profileField] = input.value; session.dirty = true; });
    el.querySelector('[data-of]').onreset = () => { sessions.delete(key); void paint(); };
    el.querySelector('[data-of]').onsubmit = async event => {
      event.preventDefault(); if (!(await requireOwner())) return;
      try { const value = await mockClient.saveAboutYou(characterId, actorId, { fields: session.fields, expectedVersion: session.base }); sessions.set(key, { fields: { ...value.fields }, base: value.version, dirty: false, conflict: false }); if (alive) await paint(); toast('Mock 资料已保存，刷新后仍可读取', 'ok'); }
      catch (error) { if (error.status === 409) { session.conflict = true; if (alive) await paint(); } else if (alive) el.querySelector('[data-result]').textContent = error.message; }
    };
    el.querySelector('[data-keep]')?.addEventListener('click', () => toast('草稿已保留，尚未提交'));
    el.querySelector('[data-refresh]')?.addEventListener('click', async () => { const value = await mockClient.aboutYou(characterId, actorId); session.base = value.version; session.conflict = false; if (alive) await paint(); });
    el.querySelector('[data-simulate-profile]').onclick = async () => { await mockClient.simulateActorConflict(characterId, actorId); toast('已模拟另一处修改；当前草稿未覆盖', ''); };
  }
  void paint().catch(error => { if (alive) el.innerHTML = `<div class="banner err">${esc(error.message)}</div>`; });
  return () => { alive = false; };
}

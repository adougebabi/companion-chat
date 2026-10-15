// studio.resource.edit —— 资源编辑（创作治理 BP-15），含助手、差异、创作预览沙盒、应用到实例
import * as store from '../store.js';
import { esc, icon, badge, drawer, toast, confirm, requireOwner, objLink } from '../ui.js';

const FIELDS = ['名称', '描述', '营业状态', '标签'];
let work = null, workFor = null, chatlog = [];
const vnum = v => +String(v).slice(1);
function base(r) { return r.draft || r.versions[r.published]; }
function diffRows(a, b) { return FIELDS.filter(f => (a[f] || '') !== (b[f] || '')).map(f => `<div class="ctx">${f}</div><div class="del">- ${esc(a[f])}</div><div class="add">+ ${esc(b[f])}</div>`).join('') || '<div class="ctx">无差异</div>'; }
function assistant(cmd) {
  const before = { ...work }; let say;
  if (/海风|气味|描述/.test(cmd)) { work.描述 = (work.描述 || '').replace(/。?$/, '') + '，推门有海风和旧书的气味。'; say = '我在「描述」末尾加了一句海风和旧书的气味。'; }
  else if (/装修|状态/.test(cmd)) { work.营业状态 = '暂停装修，预计 11 月重开'; say = '已把「营业状态」改成“暂停装修，预计 11 月重开”。'; }
  else if (/标签|安静/.test(cmd)) { work.标签 = (work.标签 ? work.标签 + ',' : '') + '安静'; say = '加上了标签「安静」。'; }
  else { say = '我可以帮你改描述、营业状态或标签，比如“描述里加上海风”。'; }
  const changed = FIELDS.filter(f => before[f] !== work[f]);
  chatlog.push({ u: cmd }, { a: say + (changed.length ? `（改动字段：${changed.join('、')}，未保存）` : '') });
}
export function renderStudioEdit(el, ctx) {
  const r = store.get('resources', ctx.params.resId);
  if (!r) { el.innerHTML = `<div class="statebox"><h2>当前范围未找到资源 ${esc(ctx.params.resId)}</h2><a class="btn" href="#/studio">回创作中心</a></div>`; return; }
  if (workFor !== r.id) { work = { ...base(r) }; workFor = r.id; chatlog = [{ a: `你好！我和左侧表单编辑的是同一份草稿（${r.name}）。试试：“描述里加上海风”。` }]; }
  const pub = r.versions[r.published]; const dirty = FIELDS.some(f => work[f] !== base(r)[f]);
  const insts = Object.entries(store.get('instances')).filter(([, v]) => v[r.id]).map(([cid, v]) => ({ cid, ...v[r.id] }));
  const drafts = store.get('studioDrafts').filter(x => x.resourceId === r.id);
  el.innerHTML = `<div class="studio"><div class="stack">
   <div class="card"><div class="row"><span class="typeicon">${icon(r.type === '地点' ? 'pin' : r.type === '物品' ? 'shirt' : 'sparkle')}</span><div class="grow"><h2 style="margin:0">${esc(r.name)}</h2><div class="small muted">${r.type} · <code>${r.id}</code> · 已发布 <b data-test="published">${r.published}</b> · ${r.draft ? badge('草稿已保存', 'plain') : ''} ${dirty ? `<span class="badge b-候选" data-test="dirty">有未保存修改</span>` : ''}</div></div>${badge('全局定义')}</div>
     <div class="small muted" style="margin-top:6px">被引用：${insts.length ? insts.map(i => `${objLink(i.cid)} 运行 ${i.applied}${Object.keys(i.overrides || {}).length ? '（有实例修正）' : ''}${i.applied !== r.published ? ' <span class="badge b-候选">待应用</span>' : ''}`).join('；') : '无实例'}</div>${store.get('meta')?.studioApplyNotes?.[r.id] ? `<div class="banner warn" data-test="apply-unfinished">${store.get('meta').studioApplyNotes[r.id]}</div>` : ''}
     <div class="banner info small" style="margin-top:8px">${icon('info')} 编辑定义不会自动改变已运行的实例，需「应用到实例」。</div></div>
   <div class="card"><h3>${icon('studio')} 定义内容（草稿）</h3><div class="stack" data-test="studio-form">${FIELDS.map(f => `<label class="field">${f}${f === '描述' ? `<textarea class="input" rows="3" data-f="${f}">${esc(work[f])}</textarea>` : `<input class="input" data-f="${f}" value="${esc(work[f])}">`}</label>`).join('')}</div></div>
   <div class="card"><h3>差异预览 <span class="small muted">已发布 ${r.published} → 当前草稿</span></h3><div class="diff" data-test="studio-diff">${diffRows(pub, work)}</div></div>
   <div class="btngroup"><button class="btn" data-save data-test="save-draft">保存草稿</button><button class="btn primary" data-pub data-test="publish">发布定义</button><button class="btn" data-apply data-test="apply">应用到实例…</button><span class="spacer"></span><button class="btn danger" data-revert>放弃修改</button></div>
   <div class="small muted">三个动作互相独立：保存草稿＝只存草稿；发布定义＝生成新版本号（全局定义）；应用到实例＝把某个已发布版本带入某个角色的运行世界（所有者修改，写审计）。</div>
  </div>
  <div class="stack"><div class="card assist" data-test="assistant"><h3>${icon('sparkle')} 创作助手 <span class="small muted">编辑同一份草稿</span></h3><div class="log">${chatlog.map(m => m.u ? `<div class="u">${esc(m.u)}</div>` : `<div class="a">${esc(m.a)}</div>`).join('')}</div>
     <div class="row">${['描述里加上海风', '营业状态改为暂停装修', '加标签 安静'].map(t => `<button class="chip" data-q="${t}">${t}</button>`).join('')}</div>
     <form class="row" data-af><input class="input grow" name="q" placeholder="让助手改草稿…" data-test="assist-input"><button class="btn accent sm">${icon('send')}</button></form></div>
   <div class="sandbox" data-test="sandbox"><div class="row"><b class="grow">试生成沙盒</b>${badge('创作预览·不进入生活')}</div><p class="small muted">结果只存在创作资源（studioDrafts），不写入任何角色的媒体库、日记、记忆或事实。</p>
     <button class="btn sm" data-gen data-test="preview-gen">${icon('sparkle')} 用当前草稿试生成</button>
     <div class="stack" style="margin-top:10px">${drafts.map(x => `<div class="card" data-test="preview-item"><div class="row">${badge('创作预览·不进入生活')}<span class="small muted grow">${x.id} · ${x.at}</span><button class="btn ghost sm" data-del="${x.id}">丢弃</button></div><img src="${x.img}" alt="" style="width:100%;border-radius:10px;margin:6px 0"><p class="small">${esc(x.text)}</p></div>`).join('')}</div>
     <p class="small muted" data-test="media-count">媒体库条目：${store.list('media').length}（试生成不会增加）</p></div></div></div>`;
  const q = s => el.querySelector(s);
  el.querySelectorAll('[data-f]').forEach(i => i.oninput = () => { work[i.dataset.f] = i.value; const d = q('[data-test=studio-diff]'); d.innerHTML = diffRows(pub, work); });
  const repaint = () => renderStudioEdit(el, ctx);
  el.querySelectorAll('[data-q]').forEach(b => b.onclick = () => { assistant(b.dataset.q); repaint(); });
  q('[data-af]').onsubmit = e => { e.preventDefault(); if (e.target.q.value.trim()) { assistant(e.target.q.value); repaint(); } };
  q('[data-revert]').onclick = () => { work = { ...base(r) }; repaint(); toast('已放弃未保存的修改'); };
  q('[data-save]').onclick = async () => { if (!(await requireOwner())) return; store.update(d => { d.resources[r.id].draft = { ...work }; }, { identity: '所有者修改', summary: `保存草稿：${r.name}（未发布）`, target: r.id }); toast('草稿已保存（未发布、未应用）', 'ok'); };
  q('[data-pub]').onclick = async () => { if (!(await requireOwner())) return; const nv = 'v' + (Math.max(...Object.keys(r.versions).map(vnum)) + 1);
    if (!(await confirm({ title: '发布定义 ' + nv, text: `发布 ${r.name} 新版本 ${nv}。已运行实例不会自动改变。` }))) return;
    store.update(d => { const x = d.resources[r.id]; x.versions[nv] = { ...work }; x.published = nv; x.draft = null; }, { identity: '所有者修改', summary: `发布定义 ${r.name} ${nv}`, target: r.id }); toast('已发布 ' + nv, 'ok'); };
  q('[data-apply]').onclick = () => applyDrawer(r.id);
  q('[data-gen]').onclick = () => { const id = 'sd_' + Date.now().toString(36); store.update(d => { d.studioDrafts.push({ id, resourceId: r.id, at: new Date().toTimeString().slice(0, 5), img: '/v3/assets/scenes/seaside.svg', text: `【试生成】${work.名称}：${work.描述}（${work.营业状态}）` }); }, { identity: '所有者修改', summary: '创作预览试生成（不进入生活）', target: r.id }); };
  el.querySelectorAll('[data-del]').forEach(b => b.onclick = () => store.update(d => { d.studioDrafts = d.studioDrafts.filter(x => x.id !== b.dataset.del); }, { identity: '所有者修改', summary: '丢弃创作预览 ' + b.dataset.del, target: r.id }));
}
/** studio.apply —— 应用到实例（抽屉） */
export function applyDrawer(rid, charId = 'c1') {
  const r = store.get('resources', rid); const inst = store.get('instances')[charId]?.[rid];
  if (!inst) return drawer({ title: '应用到实例', body: '<p class="muted">该资源未被此角色引用。</p>' });
  const from = r.versions[inst.applied]; const to = r.versions[r.published];
  const conflicts = Object.keys(inst.overrides || {}).filter(f => to[f] !== from[f]);
  const choice = Object.fromEntries(conflicts.map(f => [f, 'keep']));
  const d = drawer({ title: `${icon('swap')} 应用到实例 · studio.apply`, body: `<p class="small">${badge('全局定义')} → ${badge('角色范围')} ${objLink(charId)}：${inst.applied} → <b>${r.published}</b></p>
    ${inst.applied === r.published ? '<div class="banner ok">实例已是最新版本，无需应用。</div>' : ''}
    <h3>差异预览</h3><div class="diff">${diffRows(from, to)}</div>
    <h3 style="margin-top:12px">影响对象</h3><div class="small">${objLink('p_book')}（地点描述）· 记忆 ${objLink('m31', '#m31')} 不会被自动改写</div>
    ${conflicts.length ? `<div class="banner err" style="margin-top:12px" data-test="apply-conflict">${icon('warn')} 冲突：实例已有本地修正，请逐项选择</div>${conflicts.map(f => `<div class="card"><b>${f}</b><div class="small">实例修正：${esc(inst.overrides[f])}</div><div class="small">新定义 ${r.published}：${esc(to[f])}</div>
      <div class="row" style="margin-top:6px"><label class="toggle"><input type="radio" name="c_${f}" value="keep" checked> 保留实例修正</label><label class="toggle"><input type="radio" name="c_${f}" value="over" data-test="apply-override"> 覆盖为新定义</label></div></div>`).join('')}` : ''}`,
    footer: `<button class="btn" data-c>取消</button><button class="btn teal" data-ok data-test="apply-confirm" ${inst.applied === r.published ? 'disabled' : ''}>确认应用</button>` });
  d.el.querySelectorAll('input[type=radio]').forEach(i => i.onchange = () => choice[i.name.slice(2)] = i.value);
  d.el.querySelector('[data-c]').onclick = () => d.close();
  d.el.querySelector('[data-ok]').onclick = async () => {
    if (!(await requireOwner())) return;
    if (!(await confirm({ title: `应用 ${r.name} ${r.published} 到 ${store.get('characters', charId).name}`, text: conflicts.map(f => `${f}：${choice[f] === 'keep' ? '保留实例修正' : '覆盖为新定义'}`).join('；') || '无冲突字段' }))) return;
    try {
      await store.recordStudioApply(rid, { fluctlightId: charId, resourceVersion: r.published, expectedInstanceRevision: inst.revision || 0, choices: conflicts.map(f => ({ field: f, decision: choice[f] === 'keep' ? 'keep' : 'override' })), reason: `记录应用 ${r.name}`, idempotencyKey: store.uid('idem') });
    } catch (error) { toast(error?.userMessage || error?.message || '应用没有写入', 'err'); }
    d.close();
  };
  return d;
}

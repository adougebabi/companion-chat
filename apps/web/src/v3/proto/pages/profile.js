import {get as readStore} from '../store.js';
import { randomId } from '../../../random-id.mjs';
import { Field } from '../components/forms.js';
import { drawer } from '../components/overlay.js';
import { ProfileHeader, ProfileGroups, PersonaSection, PersonaHistory } from '../components/profile.js';
import {mountInitializationProgress} from '../components/initialization-progress.js';

/** @type {import('../data/profile-types.ts').ProfileProvider | null} */
let configuredProvider = null;
const views = new WeakMap();
/** Shared application/store bridge. Register once for the explicit real or mock mode. */
export function configureProfileProvider(provider) { configuredProvider = provider; }
export function getProfileProvider() { return configuredProvider; }

/** The page owns drafts and request lifetime only; provider owns authoritative reads/commands.
 * @param {HTMLElement} el
 * @param {{charId: string}} ctx
 * @param {import('../data/profile-types.ts').ProfileProvider | null} provider
 */
export async function renderProfilePage(el, ctx, provider = configuredProvider, options = {}) {
  const token = {}; views.set(el, token);
  const pageKey = el.dataset.page;
  const current = () => !options.signal?.aborted && views.get(el) === token && el.dataset.page === pageKey;
  let snapshot; let pending = false; let commandKey; let commandFingerprint;let progressView=null;
  options.signal?.addEventListener('abort',()=>progressView?.dispose(),{once:true});
  let draft = { targetPersonaId: '', reason: '', confirm: false };
  if (!provider) {
    el.innerHTML = '<div class="statebox"><h2>角色资料尚未连接</h2><p class="muted">无法读取当前角色的资料。</p></div>'; return;
  }
  function feedback(message, error = false) {
    if (!current()) return;
    const node = el.querySelector('[data-profile-feedback]');
    if (node) { node.className = `banner ${error ? 'err' : 'ok'}`; node.textContent = message;const panel=node.closest?.('.persona-controls');if(panel)panel.open=true; }
  }
  function paint() {
    if (!current()) return;
    progressView?.dispose();progressView=null;
    const id = encodeURIComponent(snapshot.id);
    el.innerHTML = `<div class="stack" data-test="profile-detail">${ProfileHeader(snapshot, Boolean(provider.editBio))}${provider.progress?'<section class="card stack" data-initialization-progress></section>':''}${ProfileGroups(snapshot)}${PersonaSection(snapshot, Boolean(provider.switchPersona))}${PersonaHistory(snapshot)}<div class="row"><a class="btn" href="#/c/${id}/people/owner">关于你</a>${readStore('ui').identity==='管理'?`<a class="btn" href="#/c/${id}/governance">身份与能力治理</a><a class="btn" href="#/c/${id}/worlds">世界管理</a>`:''}${snapshot.mode!=='mock'?`<a class="btn" href="#/c/${id}/media/avatar">显示头像</a>`:''}</div></div>`;
    if(provider.progress)progressView=mountInitializationProgress(el.querySelector('[data-initialization-progress]'),{fid:snapshot.id,read:provider.progress,alive:current});
    const form = el.querySelector('[data-profile-switch]');
    form.elements.targetPersonaId.value = draft.targetPersonaId;
    form.elements.reason.value = draft.reason;
    form.elements.confirm.checked = draft.confirm;
    form.onsubmit = submit;
    el.querySelector('[data-profile-refresh]').onclick = () => { if (!pending) { captureDraft(); load(true); } };
    el.querySelector('[data-profile-edit]').onclick = editBio;
  }
  function captureDraft() {
    const form = el.querySelector('[data-profile-switch]');
    if (form) draft = { targetPersonaId: form.elements.targetPersonaId.value, reason: form.elements.reason.value, confirm: form.elements.confirm.checked };
  }
  function lock(value) {
    pending = value;
    el.querySelectorAll('[data-profile-switch] input, [data-profile-switch] textarea, [data-profile-switch] select, [data-profile-switch] button, [data-profile-edit]').forEach(control => { control.disabled = value; });
  }
  async function load(keepDraft = false) {
    if (!snapshot) el.innerHTML = '<div class="statebox" role="status">正在读取角色资料…</div>';
    pending = true;
    try {
      const next = await provider.read(ctx.charId);
      if (!current()) return;
      if (!next || next.id !== ctx.charId) throw new Error('返回的角色范围不匹配');
      snapshot = next;
      if (!keepDraft) draft = { targetPersonaId: '', reason: '', confirm: false };
      paint();
    } catch (error) {
      if (!current()) return;
      if (snapshot) feedback(error?.message || '资料读取失败，已保留草稿。请重新读取后确认当前状态。', true);
      else {
        el.innerHTML = '<div class="statebox"><h2>角色资料读取失败</h2><p data-profile-error role="alert"></p><button class="btn" data-profile-retry>重试</button></div>';
        const failure = el.querySelector('[data-profile-error]'); if (failure) failure.textContent = error?.message || '读取失败';
        el.querySelector('[data-profile-retry]').onclick = () => load(keepDraft);
      }
    } finally { if (current()) pending = false; }
  }
  async function submit(event) {
    event.preventDefault();
    if (pending || !provider.switchPersona || snapshot.readOnly) return;
    captureDraft();
    if (!draft.targetPersonaId || draft.targetPersonaId === snapshot.activeProfileId || !snapshot.profiles.some(p => p.id === draft.targetPersonaId) || (snapshot.switchTargetIds && !snapshot.switchTargetIds.includes(draft.targetPersonaId))) { feedback('请选择其他可用人格。', true); formField('targetPersonaId').focus(); return; }
    if (!draft.reason.trim() || [...draft.reason.trim()].length > 1000) { feedback('请填写切换原因（最多 1000 字）。', true); formField('reason').focus(); return; }
    if (!draft.confirm) { feedback('请先勾选确认切换。', true); formField('confirm').focus(); return; }
    if (snapshot.foundationRevision === null || snapshot.runtimeRevision === null || !snapshot.activeProfileId) { feedback('版本或当前人格缺失，请重新读取后再试。', true); return; }
    const command = { targetPersonaId: draft.targetPersonaId, expectedRevision: snapshot.foundationRevision, expectedRuntimeRevision: snapshot.runtimeRevision, reason: draft.reason.trim(), confirm: true };
    const fingerprint = JSON.stringify(command);
    if (fingerprint !== commandFingerprint) { commandFingerprint = fingerprint; commandKey = randomId(); }
    lock(true); feedback('正在提交人格切换…');
    let accepted = false;
    try {
      await provider.switchPersona(ctx.charId, { ...command, idempotencyKey: commandKey }, options);
      accepted = true;
      const next = await provider.read(ctx.charId);
      if (!current()) return;
      if (!next || next.id !== ctx.charId) throw new Error('角色范围不匹配');
      snapshot = next;
      const confirmed = snapshot.activeProfileId === command.targetPersonaId;
      if (confirmed) { draft = { targetPersonaId: '', reason: '', confirm: false }; commandKey = undefined; commandFingerprint = undefined; }
      paint(); feedback(confirmed ? '人格切换已生效，已读取最新状态。' : '命令已提交，当前人格尚未确认变化。请重新读取状态。', !confirmed);
    } catch (error) {
      if (!current()) return;
      paint();
      feedback(accepted ? '命令已提交，但最新状态读取失败。请重新读取，确认前不要重复切换。' : error?.status === 409 ? '版本已变化，切换未提交。草稿已保留，请重新读取后确认。' : error?.message || '人格切换未确认成功，草稿已保留。请重新读取状态后重试。', true);
    } finally { if (current()) pending = false; }
  }
  function formField(name) { return el.querySelector('[data-profile-switch]').elements[name]; }
  function editBio() {
    if (pending || !provider.editBio || snapshot.readOnly) return;
    const d = drawer({ title: '编辑简介 · 所有者修改', body: `${Field({ label: '简介', name: 'bio', value: snapshot.bio, type: 'textarea', rows: 4, attrs: { 'data-test': 'bio-input' } })}<p class="small muted">修改角色范围的资料，会写入审计。</p><div data-bio-error role="alert"></div>`, footer: '<button class="btn" data-c data-test="bio-cancel">取消</button><button class="btn accent" data-ok data-test="bio-save">保存</button>' });
    d.el.querySelector('[data-c]').onclick = () => d.close();
    d.el.querySelector('[data-ok]').onclick = async () => {
      const bio = d.el.querySelector('[name=bio]').value.trim();
      if (!bio) { d.el.querySelector('[data-bio-error]').textContent = '请填写简介'; return; }
      const button = d.el.querySelector('[data-ok]'); button.disabled = true;
      try { await provider.editBio(ctx.charId, bio, snapshot.foundationRevision); d.close(); if (current()) await load(true); }
      catch (error) { d.el.querySelector('[data-bio-error]').textContent = error?.message || '简介保存未确认成功，内容已保留，请重试。'; }
      finally { button.disabled = false; }
    };
  }
  await load();
}

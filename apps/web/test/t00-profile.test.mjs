import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import { profileSnapshotFromDetail } from '../src/v3/proto/data/profile.js';
import { ProfileGroups, ProfileHeader, PersonaSection } from '../src/v3/proto/components/profile.js';
import { ChatPanel, GeneratedImage } from '../src/v3/proto/components/chat.js';
import { PromptChips } from '../src/v3/proto/components/forms.js';

globalThis.document = { addEventListener() {} };
const { renderProfilePage } = await import('../src/v3/proto/pages/profile.js');
function snapshot(active = 'warm', runtimeRevision = 2) {
  return profileSnapshotFromDetail({ id: 'fixture-role', status: 'active', current_revision: 4,
    identity: { name: '<角色>', background: '真实背景', timezone: 'Asia/Shanghai' },
    core_persona: { initial_state: { appearance: '旧初始外观' }, personality_system: { active_profile_id: 'old', profiles: [{ id: 'warm', name: '日常人格' }, { id: 'focused', name: '专注人格' }] } },
    personality_runtime: { active_profile_id: active, revision: runtimeRevision },
    current_state: { appearance: null, context: { location: '有效地点' } },
    working_personas: [{ id: 'warm', source_revision: 4 }], developing_self: { claims: [] },
  });
}
// Minimal DOM binding harness; markup semantics are checked separately below.
function surface() {
  const panel={open:false};
  const fields = Object.fromEntries(['targetPersonaId', 'reason', 'confirm'].map(name => [name, { value: '', checked: false, focus() { this.focused = true; } }]));
  const nodes = { '[data-profile-switch]': { elements: fields }, '[data-profile-feedback]': {closest:()=>panel}, '[data-profile-refresh]': {}, '[data-profile-edit]': {}, '[data-profile-retry]': {} };
  return { dataset: { page: 'profile.char.view' }, innerHTML: '', nodes,panel,
    querySelector(selector) { return nodes[selector]; }, querySelectorAll(selector) { return selector === '[data-avatar-image]' ? [] : Object.values(fields); } };
}
function draft(el) { const fields = el.nodes['[data-profile-switch]'].elements; fields.targetPersonaId.value = 'focused'; fields.reason.value = '专注完成当前工作'; fields.confirm.checked = true; }
const submit = el => el.nodes['[data-profile-switch]'].onsubmit({ preventDefault() {} });

test('UI-01 removes chat chips and picture send while retaining media display and assistant chips', async () => {
  const html = ChatPanel({ character: { id: 'fixture', avatar: '', name: '测试' }, summary: '', messages: [], messageOptions: {}, suggestions: ['不应出现'], sidebar: '' });
  assert.doesNotMatch(html, /data-chip|不应出现|i-image|多模态附件/);
  assert.match(html, /data-test="chat-send"/);
  assert.match(GeneratedImage({ img: '/fixture.svg', title: '生成图' }), /<img/);
  assert.match(PromptChips(['创作助手推荐']), /data-chip/);
  const source = await readFile(new URL('../src/v3/proto/pages/chat.js', import.meta.url), 'utf8');
  assert.doesNotMatch(source, /suggestions:|querySelectorAll\('\[data-chip\]/);
});

test('profile adapter preserves cleared effective appearance and runtime authority', () => {
  const value = snapshot();
  assert.equal(value.activeProfileId, 'warm');
  assert.equal(value.groups.find(g => g.key === 'current').value.appearance, null);
  assert.equal(value.foundationRevision, 4); assert.equal(value.runtimeRevision, 2);
  assert.equal(profileSnapshotFromDetail({ id: 'empty', core_persona: { personality_system: { active_profile_id: 'initial-only' } } }).activeProfileId, null);
  const html = ProfileGroups(value);
  assert.match(html, /未记录 \/ 已清除/); assert.match(html, /工作人格/);
  assert.match(ProfileHeader(value, true), /&lt;角色&gt;/);
  assert.doesNotMatch(html, /<角色>/);
});

test('unwired command visibly disables switching without fabricating a current persona', () => {
  const html = PersonaSection(snapshot(), false);
  assert.match(html, /profile-switch-unavailable/); assert.match(html, /profile-switch-submit" disabled/);
  const empty = profileSnapshotFromDetail({ id: 'empty' });
  assert.equal(empty.name, ''); assert.equal(empty.profiles.length, 0);
  assert.match(ProfileHeader(empty, false), /姓名尚未记录/);
});

test('switch sends dual revisions and idempotency then confirms only authoritative readback', async () => {
  const el = surface(); let changed = false; let sent;
  await renderProfilePage(el, { charId: 'fixture-role' }, { read: async () => snapshot(changed ? 'focused' : 'warm', changed ? 3 : 2), switchPersona: async (id, command) => { assert.equal(id, 'fixture-role'); sent = command; changed = true; } });
  draft(el); await submit(el);
  assert.equal(sent.expectedRevision, 4); assert.equal(sent.expectedRuntimeRevision, 2);
  assert.equal(sent.targetPersonaId, 'focused'); assert.equal(sent.confirm, true); assert.ok(sent.idempotencyKey);
  assert.match(el.nodes['[data-profile-feedback]'].textContent, /已生效/);
  assert.match(el.innerHTML, /专注人格/);
});

test('version conflict preserves drafts and does not claim switch success', async () => {
  const el = surface(); let calls = 0; const keys = [];
  await renderProfilePage(el, { charId: 'fixture-role' }, { read: async () => snapshot(), switchPersona: async (_, command) => { calls++; keys.push(command.idempotencyKey); throw Object.assign(new Error('conflict'), { status: 409 }); } });
  draft(el); await submit(el); await submit(el);
  assert.equal(calls, 2); assert.equal(keys[0], keys[1]);
  assert.equal(el.nodes['[data-profile-switch]'].elements.reason.value, '专注完成当前工作');
  assert.match(el.nodes['[data-profile-feedback]'].textContent, /版本已变化/);
  assert.equal(el.panel.open,true);
  assert.doesNotMatch(el.nodes['[data-profile-feedback]'].textContent, /已生效/);
});

test('accepted command with failed readback remains unconfirmed and keeps recovery draft', async () => {
  const el = surface(); let reads = 0;
  await renderProfilePage(el, { charId: 'fixture-role' }, { read: async () => { if (reads++) throw new Error('network'); return snapshot(); }, switchPersona: async () => ({ accepted: true }) });
  draft(el); await submit(el);
  assert.match(el.nodes['[data-profile-feedback]'].textContent, /命令已提交，但最新状态读取失败/);
  assert.equal(el.nodes['[data-profile-switch]'].elements.reason.value, '专注完成当前工作');
});

test('stale profile read cannot replace a later route, and initial failure exposes retry', async () => {
  const el = surface(); let resolve;
  const pending = renderProfilePage(el, { charId: 'fixture-role' }, { read: () => new Promise(done => { resolve = done; }) });
  el.dataset.page = 'life.now'; el.innerHTML = '后续页面'; resolve(snapshot()); await pending;
  assert.equal(el.innerHTML, '后续页面');
  const failing = surface(); await renderProfilePage(failing, { charId: 'fixture-role' }, { read: async () => { throw new Error('failure'); } });
  assert.match(failing.innerHTML, /data-profile-retry/);
});


test('owner persona projection limits legal switch targets and retains bounded history metadata', () => {
  const value = profileSnapshotFromDetail({ id: 'fixture', persona_compilation: { status: 'ready', working_persona: { name: '工作人格' }, owner_switch_targets: [{ target_profile_id: 'focused' }] }, persona_switch_history_has_more: true, persona_switch_history: [{ previous_profile_id: 'warm', active_profile_id: 'focused', switch_reason: '工作', revision: 3 }] });
  assert.deepEqual(value.switchTargetIds, ['focused']); assert.equal(value.historyHasMore, true);
  assert.equal(value.history[0].from_profile_id, 'warm'); assert.equal(value.history[0].to_profile_id, 'focused'); assert.equal(value.history[0].reason, '工作');
  assert.equal(value.groups.find(g => g.key === 'working').value.status, 'ready');
  const role = snapshot(); role.switchTargetIds = [];
  assert.match(PersonaSection(role, true), /profile-switch-submit" disabled/);
});

function editableDetail(status) {
  return { id: 'fixture-role', status, current_revision: 4, identity: { name: '角色', background: '简介' },
    core_persona: { personality_system: { profiles: [{ id: 'warm', name: '日常人格' }, { id: 'focused', name: '专注人格' }] } },
    personality_runtime: { active_profile_id: 'warm', revision: 2 } };
}
test('retired and archived details disable bio edit and persona switch; active stays writable', async () => {
  for (const status of ['retired', 'archived', 'active']) {
    const value = profileSnapshotFromDetail(editableDetail(status));
    assert.equal(value.status, status);
    assert.equal(value.readOnly, status !== 'active');
    const el = surface();
    await renderProfilePage(el, { charId: 'fixture-role' }, { read: async () => value, editBio: async () => {}, switchPersona: async () => {} });
    const edit = el.innerHTML.match(/data-test="profile-edit"[^>]*/)?.[0] ?? '';
    const submit = el.innerHTML.match(/data-test="profile-switch-submit"[^>]*/)?.[0] ?? '';
    if (status === 'active') { assert.doesNotMatch(edit, /disabled/); assert.doesNotMatch(submit, /disabled/); }
    else { assert.match(edit, /disabled/); assert.match(submit, /disabled/); }
  }
  assert.equal(profileSnapshotFromDetail(editableDetail('paused')).readOnly, false);
});

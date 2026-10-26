import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import { profileSnapshotFromDetail } from '../src/v3/proto/data/profile.js';
import { ProfileGroups, ProfileHeader, PersonaSection } from '../src/v3/proto/components/profile.js';
import { ChatPanel, GeneratedImage } from '../src/v3/proto/components/chat.js';
import { PromptChips } from '../src/v3/proto/components/forms.js';

globalThis.document = { addEventListener() {} };
import {nativeProfileFixture,profileDetail} from './support/native-profile-fixture.mjs';
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
  const source = await readFile(new URL('../src/approved-ui/pages/ChatPage.mjs', import.meta.url), 'utf8');
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

test('native profile sends both revisions and one idempotent switch then confirms actual current state',async()=>{
 let changed=false,sent;const f=await nativeProfileFixture({read:async()=>profileDetail({active:changed?'focused':'warm',revision:changed?3:2}),switchPersona:async(fid,body)=>{assert.equal(fid,'fixture-role');sent=body;changed=true;return {status:'completed',active_profile_id:'focused'};}});
 try{await f.read();f.draft();await f.c.switchPersona();assert.equal(sent.expectedRevision,4);assert.equal(sent.expectedRuntimeRevision,2);assert.equal(sent.targetPersonaId,'focused');assert.equal(sent.confirm,true);assert.ok(sent.idempotencyKey);assert.match(f.c.personaFeedback.value,/已生效/);assert.equal(f.c.snapshot.value.activeProfileId,'focused');assert.match(await f.html(),/专注人格/);}finally{await f.dispose();}
});
test('native profile keeps the same key, target and reason through unresolved version conflict',async()=>{
 const keys=[];const f=await nativeProfileFixture({switchPersona:async(_fid,body)=>{keys.push(body.idempotencyKey);throw Object.assign(Error('conflict'),{status:409});}});
 try{await f.read();f.draft();await f.c.switchPersona();await f.c.switchPersona();assert.equal(keys.length,2);assert.equal(keys[0],keys[1]);assert.equal(f.c.reason.value,'专注完成当前工作');assert.equal(f.c.target.value,'focused');assert.match(f.c.error.value,/版本已变化/);assert.doesNotMatch(f.c.personaFeedback.value,/已生效/);assert.match(await f.html(),/版本已变化/);}finally{await f.dispose();}
});
test('native profile retains accepted switch input on readback failure and recovers by reading without a new switch',async()=>{
 let reads=0,writes=0,recovered=false;const f=await nativeProfileFixture({read:async()=>{if(reads++&& !recovered)throw Error('network');return profileDetail({active:recovered?'focused':'warm'});},switchPersona:async()=>{writes++;return {status:'completed',active_profile_id:'focused'};}});
 try{await f.read();f.draft();await f.c.switchPersona();assert.equal(writes,1);assert.equal(f.c.personaReadbackPending.value,true);assert.equal(f.c.target.value,'focused');assert.equal(f.c.reason.value,'专注完成当前工作');assert.match(f.c.error.value,/命令已提交.*读取失败/);assert.match(await f.html(),/重新读取人格切换结果/);await f.c.switchPersona();assert.equal(writes,1);recovered=true;await f.c.readPersonaOutcome();assert.equal(writes,1);assert.equal(f.c.personaReadbackPending.value,false);assert.equal(f.c.snapshot.value.activeProfileId,'focused');}finally{await f.dispose();}
});
test('native profile discards a read after unmount and exposes a retry after an initial failure',async()=>{
 let finish;const f=await nativeProfileFixture({read:()=>new Promise(resolve=>finish=resolve)});const pending=f.read();await f.dispose();finish(profileDetail({name:'过时角色'}));await pending;assert.equal(f.c.detail.value,null);
 const next=await nativeProfileFixture({read:async()=>{throw Error('initial failure');}});try{assert.equal(await next.read(),false);assert.match(await next.html(),/重新读取/);}finally{await next.dispose();}
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
test('native profile hides retired switches and rejects direct writes while active stays editable',async()=>{
 for(const status of ['retired','archived','active']){let writes=0;const f=await nativeProfileFixture({read:async()=>profileDetail({status}),switchPersona:async()=>{writes++;return {status:'completed',active_profile_id:'focused'};}});try{await f.read();f.draft();assert.equal(f.c.snapshot.value.readOnly,status!=='active');const html=await f.html();if(status!=='active'){assert.doesNotMatch(html,/确认切换/);assert.match(html,/data-testid="detail-edit"[^>]*disabled/);f.c.edit();assert.equal(f.c.editing.value,false);await f.c.switchPersona();assert.equal(writes,0);}else{assert.match(html,/确认切换/);assert.doesNotMatch(html,/data-testid="detail-edit"[^>]*disabled/);f.c.edit();assert.equal(f.c.editing.value,true);}}finally{await f.dispose();}}
});

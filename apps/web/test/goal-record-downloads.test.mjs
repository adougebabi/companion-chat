import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => { await vite.close(); env.restore(); });
const load = path => vite.ssrLoadModule('/src/' + path);
const {configureDomainService} = await load('v3/proto/services.js');
const {_debug} = await load('v3/proto/store.js');
_debug.data.session = {actorId: 'fixture-owner'};

const timestamp = '2026-11-23T01:44:08.430+00:00';
const technical = {
  revision: 719, protocol_version: 'fluctlight.goal.v9',
  source_ref: 'res_hidden', idempotency_key: 'private-replay',
  metrics: {world_item_hidden: 5821}, correlation_id: 'req_hidden',
};
const goal = {
  ...technical, id: 'goal_fixture', fluctlight_id: 'fluctlight_hidden', target_actor_id: 'actor_hidden',
  profile_id: 'profile_hidden', status: 'active', scope: 'relationship', deadline: timestamp,
  desired_outcome: '整理街景画册', motivation: '', success_criteria: ['选出三张画作'],
  stages: [{...technical, id: 'stage_hidden', purpose: '选取草图', strategy: '按地点归类', status: 'active'}],
  commitments: [{...technical, id: 'commitment_hidden', stage_id: 'stage_hidden', expected_result: '完成图册初稿', window_start: timestamp, window_end: null}],
  execution: {...technical, stage: 'awaiting_next_step', next_step: '整理现有草图', due_at: timestamp, last_attempt: null, last_result: null},
};
const evidence = {
  ...technical, id: 'evidence_hidden', status: 'confirmed',
  source: {recorded_at: timestamp, source_data: {summary: '已选出三张画作', location: 'place_hidden'}},
};
const revision = {...technical, id: 'revision_hidden', operation: 'update', reason: '新增画册整理条件', created_at: timestamp, snapshot: goal};
const modelError = 'adk_run: [NodeRunError] provider request failed http://model.internal:9000/complete Prompt (1677 tokens) requires GPU memory';
const badRun = {...technical, id: 'goal_planner_hidden', created_at: timestamp, status: 'awaiting_trigger', error_code: modelError,
  result: {decision: 'failed', reason: modelError, details: {'未来字段': ['&<"\' 中', false, 0, null]}}};
const goodRun = {...technical, id: 'goal_planner_good', created_at: timestamp, status: 'succeeded', mode: 'apply',
  result: {decision: 'applied', goal_ids: [goal.id], reason: '已确认画册适合当前安排', review_condition: null}};
const declinedRun = {id: 'goal_planner_declined', status: 'succeeded', result: {decision: 'no_viable_candidate', reason: '目前没有合适的新方向'}};
const source = {...technical, source_id: 'source_hidden', status: 'requires_semantic_review', raw_source: [{text: '想留下一本画册'}], created_at: timestamp};
const noInternals = /尚未记录|未记录相关|时间未知|动机尚未|开始时间未限定|结束时间未限定|完整记录与来源|\b(?:claim[ _]revision|revision|protocol[ _]version|source[ _]ref|idempotency(?:[ _]key)?|metrics|correlation[ _]id|available[ _]at|created[ _]at|error[ _]code|fluctlight[ _]id|merge_window_seconds|\bv719\b)|fluctlight\.goal\.v9|res_hidden|world_item_hidden|place_hidden|req_hidden|fluctlight_hidden|actor_hidden|profile_hidden|stage_hidden|commitment_hidden|goal_planner_|5821|private-replay|2026-11-23T01:44:08|adk_run|NodeRunError|model\.internal|Prompt|GPU memory/i;
const caption = '原始规划记录（诊断）';
const button = (host, label) => [...host.querySelectorAll('button')].find(node => node.textContent.trim() === label);
const download = (host, title) => [...host.querySelectorAll('a[download]')]
  .filter(link => link.textContent === title).map(link => {
    assert.equal(link.download, 'fluctlight-record.json');
    const prefix = 'data:application/json;charset=utf-8,';
    const href = link.getAttribute('href');
    assert.ok(href.startsWith(prefix));
    return JSON.parse(decodeURIComponent(href.slice(prefix.length)));
  });
const assertClean = host => {
  const text = [host.textContent, ...[...host.querySelectorAll('input:not([type="hidden"]), textarea:not([hidden])')].map(node => node.value)].join(' ');
  assert.doesNotMatch(text, noInternals);
  assert.equal(host.querySelector('pre, code'), null);
};
async function mountLegacy(t, render, options = {}) {
  sessionStorage.clear();
  const host = document.createElement('div');
  document.body.append(host);
  const dispose = render(host, {charId: goal.fluctlight_id, ...options});
  t.after(() => { dispose(); host.remove(); });
  await new Promise(resolve => setTimeout(resolve, 0));
  return host;
}
const planningState = {
  ...technical, facts_revision: 'fact-fixture', goals: [goal], dependencies: [], source_reviews: [source],
  auto_planning_enabled: true, ordering_mode: 'automatic', max_active_goals: 5, active_count: 1,
  merge_window_seconds: 5, retry_backoff_seconds: 30, max_attempts: 3, lease_seconds: 120, candidate_limit: 10,
};

test('独立目标规划显示中文时间、涉及目标、采纳结果和安全失败提示，诊断可完整下载', async t => {
  const {renderRealGoalPlanning} = await load('v3-domain/goal-planning.js');
  let changes;
  configureDomainService('life', {
    goalSet: async () => planningState,
    planningHistory: async () => [badRun, goodRun, declinedRun],
    updateGoalSet: async (_fid, _base, value) => {
      changes = value;
      throw Object.assign(Error('fixture policy submission stopped'), {status: 422});
    },
  });
  const host = await mountLegacy(t, renderRealGoalPlanning);
  for (const expected of ['规划时间：2026-11-23 09:44 (北京时间)', '涉及目标：整理街景画册', '已采纳规划并更新目标',
    '规划原因：已确认画册适合当前安排', '本次没有可采纳的候选目标', '目前没有合适的新方向',
    '等待触发条件', '上次规划未完成，任务正在等待再次尝试。', '来源内容：想留下一本画册', '当前活跃 1',
    '合并等待时间（秒）']) assert.ok(host.textContent.includes(expected), expected);
  assert.doesNotMatch(host.textContent, /复核条件：|目标集\s*v/);
  assertClean(host);
  assert.deepEqual(download(host, caption), [badRun, goodRun, declinedRun]);
  assert.deepEqual(download(host, '原始目标来源审查记录（诊断）'), [source]);

  const form = host.querySelector('[data-policy]');
  form.elements.reason.value = '修改规划重试设置';
  form.elements.retry_backoff_seconds.value = '45';
  form.elements.retry_backoff_seconds.dispatchEvent(new window.Event('input', {bubbles: true}));
  await form.onsubmit({preventDefault() {}, target: form});
  assert.deepEqual(changes.recoveryPolicy, {
    merge_window_seconds: 5, retry_backoff_seconds: 45, max_attempts: 3, lease_seconds: 120, candidate_limit: 10,
  });
});

test('规划命令和初次读取失败只在诊断下载中保留原始报错', async t => {
  const {renderRealGoalPlanning} = await load('v3-domain/goal-planning.js');
  const cause = Object.assign(new Error(modelError), {code: 'provider_request_failed', details: technical});
  configureDomainService('life', {
    goalSet: async () => planningState,
    planningHistory: async () => [],
    plan: async () => { throw cause; },
  });
  const host = await mountLegacy(t, renderRealGoalPlanning);
  await host.querySelector('[data-plan]').onclick();
  assert.match(host.textContent, /规划服务暂时未能完成这次请求，可以稍后重试/);
  assertClean(host);
  const [record] = download(host, caption);
  assert.equal(record.message, modelError);
  assert.equal(record.stack, cause.stack);
  assert.deepEqual(record.details, technical);

  configureDomainService('life', {
    goalSet: async () => planningState,
    planningHistory: async () => { throw cause; },
  });
  const failedRead = await mountLegacy(t, renderRealGoalPlanning);
  assert.match(failedRead.textContent, /目标规划暂时无法读取/);
  assertClean(failedRead);
  assert.deepEqual(download(failedRead, caption), [record]);
});

for(const entry of ['list','object'])test(entry+'入口复用目标详情，摘要与完整记录保持一致', async () => {
  const {default: Goals} = await load('react-app/pages/GoalsPage.jsx');
  const state = proxy({session: {actorId: 'fixture-owner'}, characters: [{id: goal.fluctlight_id, status: 'active', name: '林青'}]});
  const workspace = {
    state, current: {get value() { return state.characters[0]; }},
    subscribe: listener => subscribe(state, listener), getSnapshot: () => snapshot(state),
    isCurrent: (owner, fid) => owner === state.session.actorId && fid === goal.fluctlight_id,
    client: {
      goals: async (_fid, {history}) => ({items: history ? [] : [goal], next_cursor: ''}),
      goalDetail: async () => goal,
      goalEvidence: async () => ({items: [evidence], next_cursor: ''}),
      goalHistory: async () => ({items: [revision], next_cursor: ''}),
    },
  };
  const route=entry==='object'?approvedRoute('#/c/'+goal.fluctlight_id+'/life/goal/'+goal.id):{query:new URLSearchParams()};
  if(entry==='object')assert.equal(route.feature.native,'goal');
  const view = await mountReact(vite, Goals, {workspace, props: {route}});
  const blobs = [];
  const create = URL.createObjectURL;
  const revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype;
  const click = anchor.click;
  URL.createObjectURL = blob => { blobs.push(blob); return 'blob:goal-fixture'; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { assert.equal(this.download, 'fluctlight-record.json'); };
  try {
    if(entry==='list')await view.click(view.host.querySelector('[aria-label="查看目标 整理街景画册"]'));
    const dialog = document.querySelector('[role="dialog"]');
    for (const heading of ['推进条件与核对状态', '证据来源', '目标修订历史']) await view.click(button(dialog, heading));
    const records = [
      ['原始目标阶段记录（诊断）', goal.stages[0]], ['原始行动承诺记录（诊断）', goal.commitments[0]],
      ['原始目标证据记录（诊断）', evidence], ['原始目标修订记录（诊断）', revision], ['原始目标与推进记录（诊断）', goal],
    ];
    for (const [title, expected] of records) {
      const disclosure = button(dialog, title);
      assert.ok(disclosure, title);
      await view.click(disclosure);
      await view.click(button(disclosure.closest('.product-disclosure'), '下载原始记录'));
      assert.deepEqual(JSON.parse(await blobs.at(-1).text()), expected);
    }
    assertClean(dialog);
    for (const expected of ['推进方式：按地点归类', '完成图册初稿', '窗口开始：2026-11-23 09:44 (北京时间)',
      '下一步：整理现有草图', '实际证据：已选出三张画作', '变更原因：新增画册整理条件']) {
      assert.ok(dialog.textContent.includes(expected), expected);
    }
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    anchor.click = click;
    await view.dispose();
  }
});

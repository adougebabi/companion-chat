import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get,replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const fid = 'fluctlight_xiaoman';
const worldId = 'world_xiaoman';
const actorId = 'actor_owner';
const start = '2026-11-23T03:05:00Z';
const end = '2026-11-23T03:35:00Z';
const effects = [{kind: 'weather', scope: 'place', place_id: 'place_market', value: '市集下起细雨 <短时>'}];
const template = {
  id: 'world_template_rain', name: '午后阵雨 & 微风', enabled: true, revision: 2,
  world_id: worldId, source_resource_id: 'res_rain', source_version: 'v9',
  source_ref: 'req_template_source', metrics: {ticks: 917},
  definition: {
    schema_version: 'world-event.v1', description: '阵雨从旧街经过。',
    duration_minutes: 30,
    trigger: {mode: 'automatic', interval_minutes: 60, cooldown_minutes: 120, probability: 0},
    condition: {day: 'weekend', start: '22:00', end: '05:00', require_presence: true, place_ids: ['place_market']},
    effects,
  },
  last_checked_at: start, next_check_at: end, last_triggered_at: null,
};
const disabled = {
  id: 'generated_template_wind', name: 'generated_template_wind', enabled: false, revision: 3,
  definition: {description: ' ', trigger: {}, condition: {}, effects: []},
  source_resource_id: null, next_check_at: '', last_checked_at: null,
};
const run = {
  id: 'world_event_rain', template_id: template.id, world_id: worldId, revision: 6, template_revision: 2,
  due_at: start, expires_at: end, status: 'applied', recovery_status: 'completed',
  life_event_id: 'life_event_rain', source_ref: 'req_run_source', idempotency_key: 'req_run_retry',
  result: {actual_start_at: start, actual_end_at: end, effects, metrics: {ticks: 917}},
  recovery_result: {status: 'already_resolved', id: 'life_event_recovery', evidence_refs: ['req_recovery']},
};
const filtered = {
  id: 'world_event_filtered', revision: 4, template_revision: 2, status: 'filtered',
  result: {reason: 'time_condition_not_met', source_ref: 'req_filter_evidence'}, recovery_result: {},
};
const path = `#/c/${fid}/world/settings`;
const tick = () => new Promise(resolve => setImmediate(resolve));

async function mount(t, {query = '', pending} = {}) {
  const env = installDOM();
  const previous = get();
  const host = document.body.appendChild(document.createElement('main'));
  let dispose;
  t.after(() => {
    dispose?.();
    replaceSnapshot(previous);
    env.restore();
  });
  replaceSnapshot({
    ...previous, session: {actorId}, characters: {[fid]: {id: fid, worldId}}, worlds: {[worldId]: {id: worldId}},
  });
  if (pending) sessionStorage.setItem(`fluctlight-world-event:${actorId}:${fid}:${worldId}:${template.id}`, JSON.stringify(pending));
  configureDomainService('world', {
    refresh: async charId => assert.equal(charId, fid),
    templates: async charId => {
      assert.equal(charId, fid);
      return {items: [template, disabled]};
    },
    template: async (charId, templateId) => {
      assert.equal(charId, fid);
      assert.ok([template.id, disabled.id].includes(templateId));
      return templateId === template.id ? template : disabled;
    },
    eventRuns: async (charId, templateId, cursor) => {
      assert.equal(charId, fid);
      assert.equal(cursor, '');
      return {items: templateId === template.id ? [run, filtered] : [], next_cursor: ''};
    },
    eventRun: async (charId, templateId, runId) => {
      assert.deepEqual([charId, templateId], [fid, template.id]);
      assert.ok([run.id, filtered.id].includes(runId));
      return runId === run.id ? run : filtered;
    },
  });
  const route = approvedRoute(path + query);
  assert.equal(route.feature.render, 'renderWorldSettings');
  const render = await route.feature.load();
  dispose = render(host, {charId: route.fid, params: route.feature.params, query: Object.fromEntries(route.query)});
  await tick();
  return host;
}

function assertCleanBody(host) {
  assert.doesNotMatch(host.textContent, / · v2|\bv\d+\b|版本|尚未记录|未记录相关信息|未配置|时间未知|world_|generated_template_|life_event_|place_|req_|res_|917|time_condition_not_met/i);
  assert.doesNotMatch(host.textContent, /\b(?:id|revision|source[ _]ref|idempotency[ _]key|metrics|schema[ _]version|duration[ _]minutes|evidence[ _]refs|already_resolved)\b/i);
  assert.equal(host.querySelector('details, pre, .record-facts'), null);
}

function assertDownloads(host, records) {
  const downloads = [...host.querySelectorAll('a[download]')];
  const prefix = 'data:application/json;charset=utf-8,';
  assert.equal(downloads.length, records.length);
  for (const [index, link] of downloads.entries()) {
    assert.equal(link.textContent, '原始世界事件记录（诊断）');
    assert.equal(link.download, 'fluctlight-record.json');
    const href = link.getAttribute('href');
    assert.ok(href.startsWith(prefix));
    assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), records[index]);
  }
}

test('世界事件设置首屏仅显示中文状态，打开模板可阅读中文摘要并下载完整记录', async t => {
  const host = await mount(t);
  const links = [...host.querySelectorAll('[data-select-template]')];
  assert.deepEqual(links.map(link => [...link.children].map(node => node.textContent)), [
    [template.name, '启用'], ['未命名的事件模板', '停用'],
  ]);
  assertCleanBody(host);

  links[0].click();
  await tick();
  const detail = host.querySelector('[data-template-detail]');
  assert.equal(detail.querySelector('h3').textContent, template.name);
  assert.equal(detail.querySelector('a[href="#/studio/edit/res_rain"]').textContent, '查看创作来源');
  assert.ok(detail.querySelector('[data-enabled]').checked);
  assert.match(detail.textContent, /阵雨从旧街经过。/);
  assert.match(detail.textContent, /自动检查 · 间隔 60 分钟 · 冷却 120 分钟 · 每次自动检查选中概率 0% · 持续 30 分钟/);
  assert.match(detail.textContent, /周末 · 22:00—05:00（跨夜） · 时间条件按角色时区判断 · 要求角色实际在场 · 限定已有地点/);
  assert.match(detail.textContent, /天气 · 指定地点：市集下起细雨 <短时>/);
  assert.match(detail.textContent, /最近检查 2026-11-23 11:05 \(北京时间\)/);
  assert.doesNotMatch(detail.textContent, /最近触发/);
  assertCleanBody(host);
  assertDownloads(detail, [template]);
});

test('执行历史和详情显示中文状态、判定与恢复信息，诊断下载保留嵌套记录', async t => {
  const host = await mount(t, {query: `?template=${template.id}&run=${run.id}`});
  const list = host.querySelector('[data-run-list]');
  const detail = host.querySelector('[data-run-detail]');
  assert.deepEqual([...list.querySelectorAll('a b')].map(node => node.textContent), ['已应用', '条件拦截，未应用']);
  assert.match(list.textContent, /拦截原因 时段条件未满足/);
  assert.match(detail.textContent, /窗口开始 2026-11-23 11:05 \(北京时间\)/);
  assert.match(detail.textContent, /实际结束 2026-11-23 11:35 \(北京时间\)/);
  assert.match(detail.textContent, /天气 · 指定地点：市集下起细雨/);
  assert.match(detail.textContent, /恢复状态 已恢复/);
  assert.match(detail.textContent, /原效果已解除/);
  assert.equal(detail.querySelector(`a[href="#/c/${fid}/world/event/${run.life_event_id}"]`).textContent, '查看实际生活事件');
  assertCleanBody(host);
  assertDownloads(detail, [run]);

  list.querySelector(`[data-open-run="${filtered.id}"]`).click();
  await tick();
  assert.match(detail.textContent, /拦截原因 时段条件未满足/);
  assert.doesNotMatch(detail.textContent, /窗口开始|窗口截止|处理时间|实际结束|恢复状态|查看实际生活事件/);
  assertCleanBody(host);
  assertDownloads(detail, [filtered]);
});

test('空业务字段省略，待重试提交保留业务摘要及完整诊断下载', async t => {
  const pending = {
    idempotencyKey: 'req_pending_retry', version: 1, attempts: 1,
    value: {
      operation: 'trigger', template: template.id,
      body: {reason: '核对阵雨效果', scheduledAt: start, expectedRevision: 2},
    },
  };
  const host = await mount(t, {query: `?template=${disabled.id}`, pending});
  const detail = host.querySelector('[data-template-detail]');
  assert.equal(detail.querySelector('h3').textContent, '未命名的事件模板');
  assert.doesNotMatch(detail.textContent, /查看创作来源|最近检查|下次检查|间隔|冷却|概率|整个应用世界|尚无描述|效果记录/);
  assertCleanBody(host);
  assertDownloads(detail, [disabled]);

  host.querySelector(`[data-select-template="${template.id}"]`).click();
  await tick();
  const retry = detail.querySelector('[data-original]').parentElement;
  assert.match(retry.textContent, /受控触发 · 计划触发 2026-11-23 11:05 \(北京时间\) · 原因 核对阵雨效果/);
  assertCleanBody(host);
  assertDownloads(retry, [pending]);
});

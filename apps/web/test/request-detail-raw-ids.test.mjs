import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {JSDOM} from 'jsdom';
import {createServer} from './support/module-server.mjs';

const vite = await createServer({
  root: fileURLToPath(new URL('../',import.meta.url)),
  configFile: false,
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true, ws: false},
});
test.after(() => vite.close());
const {featureRoute} = await vite.ssrLoadModule('/src/approved-ui/feature-routes.mjs');
const {configureDomainService} = await vite.ssrLoadModule('/src/v3/proto/services.js');
const title = '原始请求记录（诊断）';
const rawError = '评估失败 Error: http://worker.internal:9200/private\n    at processRequest (worker.js:71:2)';
const request = {
  id: 'req_hidden',
  kind: 'wardrobe_wear',
  fluctlight_id: 'fluctlight_hidden',
  status: 'failed',
  revision: 2,
  created_at: '2026-11-20T01:37:00Z',
  updated_at: '2026-11-20T01:42:00Z',
  decision_reason: rawError,
  payload: {item_id: 'world_item_hidden', item_name: '浅蓝外套', reason: '准备出门', idempotency_key: 'private-replay'},
  result: {error_code: 'owner_life_assessment_failed', reason: rawError, metrics: {count: 19781}},
  result_refs: ['res_hidden'],
  source_ref: 'actor_hidden',
  correlation_id: 'private-trace',
  tasks: [{task_id: 'intent_hidden', kind: 'cognition_intent', status: 'completed'}],
  future_field: {sample: ["<&'\"中文", false, 0, null]},
};

async function mountRequest(t, record) {
  const route = featureRoute('/diag/request/' + record.id,new URLSearchParams());
  assert.equal(route.render,'renderRealRequest');
  configureDomainService('operations',{request: async id => {
    assert.equal(id,record.id);
    return record;
  }});
  const dom = new JSDOM('<main></main>');
  const host = dom.window.document.querySelector('main');
  const render = await route.load();
  const dispose = render(host,route);
  t.after(() => { dispose(); dom.window.close(); });
  await new Promise(resolve => setImmediate(resolve));
  return host;
}

function assertOnlyDownload(host, record) {
  const links = host.querySelectorAll('a[download]');
  assert.equal(links.length,1);
  const [link] = links;
  assert.equal(link.textContent,title);
  assert.equal(link.download,'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = link.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))),record);
  assert.equal(host.querySelector('details, pre, code'),null);
  assert.doesNotMatch(host.textContent,/尚未记录|未记录相关|尚无处理结论|时间未知|完整记录与来源|\b(?:id|v2|failed|completed|processing|awaiting_goal|revision|idempotency(?:_key)?|metrics|source_ref|correlation_id|created_at|updated_at|error_code|task_id|result_refs|future_field)\b|req_|fluctlight_hidden|world_item_hidden|res_hidden|actor_hidden|intent_hidden|private-replay|private-trace|19781|2026-11-20T01:/i);
}

test('请求失败显示中文摘要和北京时间，诊断链接可用且下载保留完整原记录',async t => {
  const host = await mountRequest(t,request);
  assert.match(host.textContent,/换装请求\s+执行失败/);
  assert.match(host.textContent,/对象 浅蓝外套/);
  assert.match(host.textContent,/提交理由 准备出门/);
  assert.match(host.textContent,/提交时间 2026-11-20 09:37 \(北京时间\)/);
  assert.match(host.textContent,/完成时间 2026-11-20 09:42 \(北京时间\)/);
  assert.match(host.textContent,/失败原因 本次请求未能完成判断/);
  assert.doesNotMatch(host.textContent,/评估失败|Error|worker\.internal|processRequest|worker\.js|owner_life_assessment_failed/);
  assert.equal(host.querySelector('a[href="#/diag/task/intent_hidden"]').textContent,'回应处理 · 已完成');
  assert.equal(host.querySelector('a[href="#/diag/events?eventType=owner.life.request.assessment_failed&q=req_hidden"]').textContent,'查看本次判断失败原因');
  assertOnlyDownload(host,request);
});

test('旧换装请求可补齐物品名称，处理中不把更新时间当成完成时间',async t => {
  const record = {...request,status:'processing',decision_reason:null,result:null,payload:{item_id:'world_item_hidden'}};
  const reads = [];
  configureDomainService('inventory',{read:async (fid,id) => {
    reads.push([fid,id]);
    return {name:id,description:'棕色短靴'};
  }});
  const host = await mountRequest(t,record);
  assert.deepEqual(reads,[[record.fluctlight_id,record.payload.item_id]]);
  assert.match(host.textContent,/换装请求\s+处理中/);
  assert.match(host.textContent,/对象 棕色短靴/);
  assert.doesNotMatch(host.textContent,/完成时间|09:42|处理结论|失败原因/);
  assertOnlyDownload(host,record);
});

test('日程建议显示实际结果，缺失字段和无法解析的内部名称省略',async t => {
  const record = {...request,kind:'schedule_item_suggestion',status:'completed',created_at:null,
    completed_at:'2026-11-20T02:15:00Z',updated_at:'2026-11-20T02:42:00Z',
    payload:{target_name:'req_hidden',activity:'整理读书笔记',reason:''},
    decision_reason:'The requested change was committed',result:{summary:'读书笔记已加入日程'}};
  const host = await mountRequest(t,record);
  assert.match(host.textContent,/日程建议\s+已完成/);
  assert.match(host.textContent,/对象 整理读书笔记/);
  assert.match(host.textContent,/处理结论 请求记录显示所需变更已提交/);
  assert.match(host.textContent,/实际结果 读书笔记已加入日程/);
  assert.match(host.textContent,/完成时间 2026-11-20 10:15 \(北京时间\)/);
  assert.doesNotMatch(host.textContent,/提交时间|提交理由|10:42/);
  assertOnlyDownload(host,record);
});

test('换装结果保留实际换下和换上的衣物说明',async t => {
  const record = {...request,status:'completed',
    decision_reason:'The requested change was committed',
    result:{effects:[{capability_name:'wardrobe.wear',status:'completed',call_id:'internal-call',
      output:{before_items:[{description:'旧外套'}],items:[{description:'浅蓝外套'}],revision:2}}]}};
  const host = await mountRequest(t,record);
  assert.match(host.textContent,/换装前 旧外套/);
  assert.match(host.textContent,/换装后 浅蓝外套/);
  assert.doesNotMatch(host.textContent,/call_id|internal-call|before_items|wardrobe\.wear/);
  assertOnlyDownload(host,record);
});

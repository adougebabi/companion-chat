import test from 'node:test';
import assert from 'node:assert/strict';
import {modelRouteLabel, modelRouteGroups} from '../src/approved-ui/model-route-presentation.mjs';

const cases = [
  ['conversation_cognition', '', '对话理解 · 全部任务'],
  ['conversation_cognition', 'conversation_turn_response', '对话理解 · 回合回应'],
  ['conversation_reply', 'private_reply', '私聊回复'],
  ['conversation_reply', 'proactive', '主动私聊'],
  ['conversation_reply', 'moment', '动态'],
  ['conversation_summary', 'conversation_summary_v1', '对话摘要'],
  ['diary_material', 'diary_material_v1', '日记选材'],
  ['media_prompt', 'media_prompt_normalized', '媒体提示词整理'],
  ['persona_compilation', '', '人格资料整理 · 全部任务'],
  ['reflection', 'reflection_proposal_v2', '记忆与自我反思'],
  ['diary_material_v1', '', '日记选材 · 全部任务'],
  ['media_prompt_normalized', '', '媒体提示词整理 · 全部任务'],
  ['unknown_agent', 'unknown_variant', '其他任务 · 其他变体'],
  ['conversation_reply', 'future_variant', '聊天回复 · 其他变体'],
  ['unknown_agent', '', '其他任务 · 全部任务'],
  ['toString', 'constructor', '其他任务 · 其他变体'],
];

test('模型任务名称使用中文，并为未知任务和变体提供中文兜底', () => {
  for (const [agent_id, variant, expected] of cases) {
    const route = {agent_id, variant};
    assert.equal(modelRouteLabel(route), expected);
    assert.deepEqual(route, {agent_id, variant}, '展示转换不改写业务数据');
  }
});

test('覆盖选项按业务分组，只包含可写路由并保留原始保存值', () => {
  const routes = cases.map(([agent_id, variant], index) => ({agent_id, variant, key:`agent:${agent_id}:${variant}:${index}`, writable:true}));
  const before = structuredClone(routes);
  const groups = modelRouteGroups([...routes, {agent_id:'vision_understanding', key:'read-only', writable:false}, {agent_id:'reflection', writable:true}]);
  assert.deepEqual(groups.map(group=>group.label), ['对话与交流','记忆与日记','人物与生活','媒体与创作','其他任务']);
  assert.equal(groups.flatMap(group=>group.options).length, routes.length);
  for (const route of routes) {
    assert.equal(groups.flatMap(group=>group.options).find(option=>option.key===route.key)?.label, modelRouteLabel(route));
  }
  assert.ok(groups.find(group=>group.label==='记忆与日记').options.some(option=>option.key.includes('diary_material_v1')));
  assert.deepEqual(routes, before);
  assert.deepEqual(modelRouteGroups([]), []);
});

test('真实管理页面显示分组中文选项，选择后标题保持中文且不触发写入', async () => {
  const {createServer} = await import('./support/module-server.mjs');
  const {fileURLToPath} = await import('node:url');
  const {proxy, snapshot, subscribe} = await import('valtio/vanilla');
  const {mountReact, installDOM} = await import('./helpers/react-dom.mjs');
  const environment = installDOM();
  const vite = await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  let view;
  try {
    const {default:Management} = await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
    const routes = cases.slice(0,-1).map(([agent_id,variant],index)=>({agent_id,variant,key:`agent:${agent_id}:${index}`,writable:true,source:'unconfigured',state:'unconfigured'}));
    routes.push({agent_id:'vision_understanding',key:'read-only',writable:false});
    const state = proxy({session:{actorId:'test-owner'},managementTab:'配置中心',managementSection:'模型用途',characters:[],operations:[],runtimeCatalog:{}});
    const workspace = {state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{configurationClient:{read:async()=>({endpoints:[],bindings:[],versions:{},effective_values:{},agent_model_routes:routes}),catalog:async()=>({agents:[]})},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
    view = await mountReact(vite,Management,{workspace});
    await view.click([...view.host.querySelectorAll('button')].find(node=>node.textContent.trim()==='按 Agent 或任务变体覆盖'));
    const trigger = view.host.querySelector('[aria-label="选择 Agent 模型任务"]');
    await view.click(trigger);
    const list = document.querySelector('[role="listbox"]');
    assert.deepEqual([...list.querySelectorAll('[data-slot="select-label"]')].map(node=>node.textContent),['对话与交流','记忆与日记','人物与生活','媒体与创作','其他任务']);
    assert.equal(list.querySelectorAll('[role="option"]').length,routes.length);
    assert.doesNotMatch(list.textContent,/conversation_|private_reply|unknown_|media_prompt|diary_material|agent:/);
    await view.click([...list.querySelectorAll('[role="option"]')].find(node=>node.textContent==='私聊回复'));
    assert.equal(trigger.textContent,'私聊回复');
    assert.ok(view.host.querySelector('section[aria-label="私聊回复"]'));
    assert.doesNotMatch(view.host.textContent,/conversation_cognition|conversation_reply|private_reply|agent:/);
    await view.click(trigger);
    await view.click([...document.querySelectorAll('[role="option"]')].find(node=>node.textContent==='请选择任务'));
    assert.equal(view.host.querySelector('section[aria-label="私聊回复"]'),null);
  } finally {
    if(view)await view.dispose();
    await vite.close();
    environment.restore();
  }
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { proxy, snapshot, subscribe } from 'valtio/vanilla';
import { act, installDOM, mountReact } from './helpers/react-dom.mjs';
import { creationFixture, validAnalysis } from './support/creation-fixture.mjs';
import { peopleFixture } from './fixtures/people-fixture.mjs';
import { worldFixture } from './fixtures/world-fixture.mjs';

const env = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../', import.meta.url)), appType:'custom', logLevel:'silent', server:{middlewareMode:true}});
const load = path => vite.ssrLoadModule('/src/' + path);
const { default: Preview } = await load('react-app/shared/CreationPreviewValue.jsx');
const { default: Wizard } = await load('react-app/views/CharacterWizard.jsx');
const { default: Goals } = await load('react-app/pages/GoalsPage.jsx');
const { default: Library } = await load('react-app/pages/LibraryPage.jsx');
const { default: People } = await load('react-app/pages/PeoplePage.jsx');
const { default: World } = await load('react-app/pages/WorldPage.jsx');
const { createWorkspace } = await load('state/workspace.mjs');
const store = await load('v3/proto/store.js');
test.after(async () => {await vite.close(); env.restore();});
const button = (root, text) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === text);
async function tab(v, text) {
  await act(async () => [...v.host.querySelectorAll('[role=tab]')].find(node => node.textContent.trim().startsWith(text))
    .dispatchEvent(new window.MouseEvent('mousedown', {bubbles:true, button:0, ctrlKey:false})));
  await v.flush();
}
const card = (v, title) => [...v.host.querySelectorAll('.creation-structured')].find(node => node.querySelector('h2')?.textContent === title);
function assertLeaf(node, title) {
  assert.ok(node, title);
  assert.equal(node.dataset.emptyVariant, 'page');
  assert.ok(node.querySelector('svg.lucide-leaf'));
  assert.equal(node.classList.contains('bg-card'), false);
  assert.equal(node.classList.contains('border'), false);
}
function workspace({empty = false, failGoals = false} = {}) {
  const state = proxy({ready:true, session:{actorId:'r22-empty-owner'}, selectedId:empty?'':'A', characters:empty?[]:[{id:'A',name:'雨禾',status:'正常'}], errors:{}, loading:{}});
  return {state, current:{get value(){return state.characters[0];}}, subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),
    isCurrent:(owner,fid)=>owner===state.session.actorId&&fid===state.selectedId,
    client:{goals:async()=>{if(failGoals)throw Error('读取失败');return {items:[],next_cursor:''};}},
    services:{configurationClient:{read:async()=>({bindings:[]})}},load:async()=>{}};
}
async function fixture(Component, api, owner, page) {
  sessionStorage.clear();
  const w = createWorkspace();
  store.replaceSnapshot({...store.get(),session:{actorId:owner},characters:{A:{id:'A',name:'雨禾',status:'active'}},ui:{charId:'A'}});
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, 'http://r22.test');
    let response;
    const handled = await api.handler({url,path:url.pathname,method:init.method||'GET',body:init.body?JSON.parse(init.body):{},
      json:(_res,value,status=200)=>response=new Response(JSON.stringify(value),{status,headers:{'content-type':'application/json'}})});
    assert.ok(handled, `${init.method||'GET'} ${url.pathname}`);
    return response;
  };
  const v = await mountReact(vite,Component,{workspace:w,props:{route:{page,fid:'A',path:'/c/A/'+(page==='世界'?'worlds':'people'),query:new URLSearchParams()}}});
  return {v,w,async dispose(){await v.dispose();w.dispose();}};
}

test('r22 A3 preview names style, taste, commitments, relationship seeds and media fields in Chinese without changing data', async () => {
  const value = {
    style_preferences:{style:'简约',fit:'宽松'}, preferences:{food:'清淡',drinks:'热茶'},
    recurring_commitments:[{description:'一起散步',frequency:'weekly'}],
    relationship_seeds:[{type:'friend',relationship:'大学室友'}],media_preferences:{music:'民谣'},
    future_alpha:false,future_beta:0,description:'default friend weekly',特别爱好:'收集雨声',
  };
  const before = JSON.stringify(value),v = await mountReact(vite,Preview,{props:{value}});
  try {
    for (const text of ['风格简约','版型宽松','饮食偏好清淡','饮品偏好热茶','频率每周','类型朋友','关系说明大学室友','音乐偏好民谣']) assert.ok(v.host.textContent.includes(text),text);
    const labels=[...v.host.querySelectorAll('dt')].map(node=>node.textContent);
    assert.doesNotMatch(v.host.textContent,/补充设定|频率weekly|类型friend/);
    assert.equal(new Set(labels.filter(label=>label.startsWith('其他设定 '))).size,2);
    assert.match(v.host.textContent,/否/);assert.match(v.host.textContent,/0/);
    assert.match(v.host.textContent,/default friend weekly/,'authored prose stays intact');
    assert.match(v.host.textContent,/特别爱好收集雨声/,'original Chinese field names stay intact');
    assert.equal(JSON.stringify(value),before);
  } finally {await v.dispose();}
});

test('r22 A3 single persona omits empty default blocks and inferred relationship rows while retaining explicit source and edits', async () => {
  const analysis=validAnalysis();
  analysis.core_persona.identity.name='雨禾';
  analysis.core_persona.personality_system={mode:'single',active_profile_id:'default',profiles:[],switching:{rules:[]},influence:{edges:[]},core_relationship:null,core_conflict:null,forced_activation:{},takeover_rules:[]};
  analysis.initial_relationships=[{target_actor_id:'owner',trend:'stable',role:{label:'friend'},summary:'室友'}];
  const f=creationFixture(async()=>analysis);
  f.c.draft.mode='json';f.c.draft.json=JSON.stringify({core_persona:{identity:{name:'雨禾'}},initial_relationships:[{role:'friend',summary:'室友'}]});
  await f.c.analyze();
  const m={c:f.c,draft:f.c.draft,step:'preview',currentStep:2,busy:false,pending:null,previewFields:f.c.previewFields.value,error:'',fieldErrors:{},groups:[],existingWorlds:[],worlds:[],templates:[],canContinue:true,goStep(){},configHref:()=> '#/settings'};
  const v=await mountReact(vite,Wizard,{props:{model:m}});
  try {
    const before=JSON.stringify(f.c.previewDefinition());
    await tab(v,'人格与关系');
    assert.equal(card(v,'人格档案与切换规则'),undefined);
    const relation=card(v,'初始关系');
    assert.match(relation.textContent,/朋友/);
    assert.doesNotMatch(relation.textContent,/关系对象|趋势|稳定|friend/);
    assert.equal(JSON.stringify(f.c.previewDefinition()),before);
    const next=f.c.previewDefinition();
    next.core_persona.personality_system.profiles=[{id:'default',name:'日常人格',voice:{tone:'gentle'},secrets:null,fears:[],extensions:{empty:null,zero:0,enabled:false}}];
    f.c.updatePreviewDefinition(next);
    await v.update({model:{...m,previewFields:f.c.previewFields.value}});
    const persona=card(v,'人格档案与切换规则');
    assert.match(persona.textContent,/默认人格/);assert.match(persona.textContent,/日常人格/);assert.match(persona.textContent,/温和/);
    assert.doesNotMatch(persona.textContent,/尚未记录|补充设定|default/);
    assert.match(persona.textContent,/0/);assert.match(persona.textContent,/否/);
    assert.doesNotMatch(card(v,'初始关系').textContent,/关系对象|趋势/,'unrelated edits do not adopt inferred relationship facts');
    await v.click(button(card(v,'初始关系'),'编辑内容'));
    await v.click(button(document.querySelector('[role=dialog]'),'应用修改'));
    assert.match(card(v,'初始关系').textContent,/关系对象你/,'explicit relationship document remains visible');
    assert.deepEqual(f.c.previewDefinition(),next);
  } finally {await v.dispose();f.c.dispose();}
});

for (const mode of ['description','template']) test(`r22 A3 ${mode} preview keeps unproven relationship defaults out until explicit source adoption`, async () => {
  const analysis=validAnalysis();
  analysis.core_persona.identity.name='雨禾';
  analysis.initial_relationships=[{target_actor_id:'owner',trend:'stable',role:{label:'friend'},summary:'室友'}];
  const f=creationFixture(async()=>analysis);
  f.c.draft.mode=mode;f.c.draft.bio='一位喜欢散步的朋友';
  f.c.draft.templateId='character-template';f.c.draft.templateVersion='v1';
  await f.c.analyze();
  assert.equal(f.c.step.value,'preview');
  const before=JSON.stringify(f.c.previewDefinition());
  const m={c:f.c,draft:f.c.draft,step:'preview',currentStep:2,busy:false,pending:null,previewFields:f.c.previewFields.value,error:'',fieldErrors:{},groups:[],existingWorlds:[],worlds:[],templates:[],canContinue:true,goStep(){},configHref:()=> '#/settings',showError:message=>assert.fail(message)};
  const v=await mountReact(vite,Wizard,{props:{model:m}});
  try {
    await tab(v,'人格与关系');
    assert.match(card(v,'初始关系').textContent,/朋友/);
    assert.doesNotMatch(card(v,'初始关系').textContent,/关系对象|趋势|稳定/);
    assert.equal(JSON.stringify(f.c.previewDefinition()),before,'reading never changes the normalized command source');
    if(mode==='description') {
      await v.click(button(card(v,'初始关系'),'编辑内容'));
      const editor=document.querySelector('[role=dialog]');
      assert.match(editor.querySelector('textarea').value,/"target_actor_id": "owner"/);
      assert.match(editor.querySelector('textarea').value,/"trend": "stable"/);
      await v.click(button(editor,'应用修改'));
    } else {
      await tab(v,'完整设定');
      await v.input(v.host.querySelector('[aria-label="完整设定 JSON"]'),JSON.stringify(f.c.previewDefinition(),null,2)+'\n');
      await v.update({model:m});
      await v.click(button(v.host,'应用 JSON 修改'));
      await tab(v,'人格与关系');
    }
    assert.match(card(v,'初始关系').textContent,/关系对象你/);
    assert.match(card(v,'初始关系').textContent,/趋势稳定/);
    assert.equal(JSON.stringify(f.c.previewDefinition()),before,'explicit adoption changes reading provenance only');
  } finally {await v.dispose();f.c.dispose();}
});

test('r22 B unknown relationship titles are Chinese in main copy, basis Sheet and revision Sheet with originals downloadable',async()=>{
  const api=peopleFixture({owner:'r22-people-owner'}),s=api.state('A');
  s.relations[0].role={label:'unknown'};
  s.relationHistory[0].role={label:'unknown'};
  const f=await fixture(People,api,'r22-people-owner','人物关系'),{v}=f;
  try {
    await tab(v,'你们的关系');
    assert.match(v.host.textContent,/你们的关系，从认真倾听开始/);
    assert.doesNotMatch(v.host.textContent,/unknown/);
    await tab(v,'关于你');
    await v.click(button(v.host,'关系与交往依据'));
    let sheet=document.querySelector('[role=dialog]');
    assert.match(sheet.textContent,/关系称呼关系尚未确认/);assert.doesNotMatch(sheet.textContent,/unknown/);
    const original=JSON.parse(decodeURIComponent(sheet.querySelector('a[download="people-source.json"]').href.split(',')[1]));
    assert.equal(original.role.label,'unknown');
    await act(async()=>sheet.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();
    await v.click(button(v.host,'查看修订记录'));
    sheet=document.querySelector('[role=dialog]');
    assert.match(sheet.textContent,/关系修订记录/);assert.match(sheet.textContent,/关系称呼关系尚未确认/);assert.doesNotMatch(sheet.textContent,/unknown/);
    assert.equal(s.relations[0].role.label,'unknown');
  } finally {await f.dispose();}
});

test('r22 D goals confirmed empty uses borderless Leaf and read failure retains recovery instead of empty',async()=>{
  let v=await mountReact(vite,Goals,{workspace:workspace(),props:{route:{query:new URLSearchParams()}}});
  try {assertLeaf(v.host.querySelector('[data-empty-variant]'),'goal empty');assert.match(v.host.textContent,/这一页暂时没有目标/);} finally {await v.dispose();}
  v=await mountReact(vite,Goals,{workspace:workspace({failGoals:true}),props:{route:{query:new URLSearchParams()}}});
  try {assert.equal(v.host.querySelector('[data-empty-variant=page]'),null);assert.match(v.host.textContent,/读取失败/);assert.ok(button(v.host,'重新读取当前目标'));} finally {await v.dispose();}
});

test('r22 D character library true empty uses borderless Leaf and keeps working creation/configuration links',async()=>{
  sessionStorage.clear();localStorage.clear();
  const w=workspace({empty:true}),v=await mountReact(vite,Library,{workspace:w});
  try {
    assertLeaf(v.host.querySelector('[data-empty-variant]'),'library empty');
    assert.match(v.host.textContent,/还没有一起生活的人/);
    assert.equal(v.host.querySelector('[data-testid=library-create-first]').getAttribute('href'),'#/library/create');
    assert.ok([...v.host.querySelectorAll('a')].some(node=>node.textContent==='先连接模型'));
    await act(async()=>{w.state.loading.library=true;});await v.flush();
    assert.equal(v.host.querySelector('[data-empty-variant]'),null);
  } finally {await v.dispose();}
});

test('r22 D relationships confirmed empty uses borderless Leaf',async()=>{
  const api=peopleFixture({owner:'r22-people-empty',scenario:'empty'}),f=await fixture(People,api,'r22-people-empty','人物关系');
  try {assertLeaf(f.v.host.querySelector('[data-empty-variant]'),'people empty');assert.match(f.v.host.textContent,/这个世界还没有已读人物/);} finally {await f.dispose();}
});

test('r22 E current world record displays Chinese labels, hides internal IDs and downloads the complete unchanged record',async()=>{
  const api=worldFixture({owner:'r22-world-owner',projectionReads:true}),s=api.state('A');
  const source=s.worlds[0];
  source.active=true;source.timezone='Asia/Shanghai';
  const before=JSON.stringify(source),f=await fixture(World,api,'r22-world-owner','世界'),{v}=f;
  try {
    await v.click(v.host.querySelector('[aria-label="进入世界 温和魔法世界"]'));
    await tab(v,'世界设定');
    await v.click(button(v.host,'完整世界资料与来源'));
    assert.equal(v.host.querySelector('a[download="world-source.json"]'),null,'source is collapsed by default');
    await v.click(button(v.host,'当前世界完整记录'));
    const download=v.host.querySelector('a[download="world-source.json"]'),record=download.closest('[data-slot=collapsible-content]');
    for(const label of ['世界名称','世界背景','世界氛围','基础规则','物品目录','地点分类','是否启用','北京时间'])assert.ok(record.textContent.includes(label),label);
    assert.doesNotMatch(record.textContent,/fluctlight|source_resource|venue_categories|A-world-one|world-resource|customKey|preservedValue/);
    assert.deepEqual(JSON.parse(decodeURIComponent(download.href.split(',')[1])),source);
    assert.equal(JSON.stringify(source),before);
  } finally {await f.dispose();}
});

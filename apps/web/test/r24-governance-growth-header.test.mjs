import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),cacheDir:'/workspace/r24p2/tmp/vite-governance-header',envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule(path);
test.after(async()=>{await vite.close();env.restore();});
const scenarios=[['capabilities','能力申请与审核','profile','GovernancePage'],['foundation','身份核心与提议','profile','GovernancePage'],['developing','发展中的自我','growth/self','GovernancePage'],['self','成长认识','growth','SelfPage'],['snapshot','人格快照','growth','SelfPage']];
for(const [mode,title,back,component] of scenarios)test(`${title}复用统一页头，返回在上，刷新只有图标且执行读取`,async()=>{
 const {headerWorkspace}=await load('/test/fixtures/governance-header-fixture.mjs'),w=headerWorkspace();
 const {default:Page}=await load(`/src/react-app/pages/${component}.tsx`);
 const query=new URLSearchParams(mode==='snapshot'?'view=snapshot':'tab='+mode);
 const v=await mountReact(vite,Page,{workspace:w,props:{route:{fid:'A',query,feature:{params:{}}}}});
 try {
  const h=v.host.querySelector('.record-page-header'),heading=h.querySelector('.replica-page-heading');
  assert.equal(v.host.querySelectorAll('h1').length,1);assert.equal(h.querySelector('h1').textContent,title);
  assert.ok(heading.querySelector('.replica-eyebrow').textContent);assert.ok(heading.querySelector('p').textContent);
  const link=h.querySelector('nav a');assert.equal(link.getAttribute('href'),'#/c/A/'+back);
  assert.ok(link.compareDocumentPosition(heading)&window.Node.DOCUMENT_POSITION_FOLLOWING);
  const reload=heading.querySelector('button');assert.equal(reload.getAttribute('aria-label'),'重新读取'+title);
  assert.equal(reload.dataset.size,'icon');assert.equal(reload.textContent,'');assert.ok(reload.querySelector('svg'));
  await act(async()=>reload.focus());await v.flush();assert.equal(document.querySelector('[role=tooltip]')?.textContent,'重新读取'+title);
  const before=w.calls.length;await v.click(reload);assert.ok(w.calls.length>before);
  assert.doesNotMatch(h.textContent,/header-owner|header-scope|Internal Server Error/);
  if(mode==='foundation'){const propose=[...v.host.querySelectorAll('button')].find(n=>n.textContent==='创建修订提议');assert.ok(propose.classList.contains('w-fit'));assert.ok(propose.classList.contains('justify-self-start'));}
 }finally{await v.dispose();}
});
test('刷新期间禁用按钮，失败保留中文说明且不泄漏原始报错',async()=>{
 const {default:Shell}=await load('/src/react-app/shared/RecordShell.tsx');let calls=0;
 const v=await mountReact(vite,Shell,{props:{title:'成长认识',description:'回顾成长',read:{loading:true,reload:async()=>calls++}}});
 try{assert.equal(v.host.querySelector('button').disabled,true);assert.equal(calls,0);}finally{await v.dispose();}
 const error=await mountReact(vite,Shell,{props:{title:'人格快照',read:{error:'Internal Server Error private-id',loading:false,reload:async()=>calls++}}});
 try{assert.match(error.host.textContent,/当前资料尚未完整读到/);assert.doesNotMatch(error.host.textContent,/Internal Server Error|private-id/);await error.click(error.host.querySelector('button'));assert.equal(calls,1);}finally{await error.dispose();}
});
test('规划页在数据加载前也有统一页头；图标刷新转发给原领域控制器',async()=>{
 const {default:Page}=await load('/src/react-app/pages/DomainPage.tsx');
 const {headerWorkspace}=await load('/test/fixtures/governance-header-fixture.mjs'),w=headerWorkspace();let reads=0;
 const {client}=await load('/src/v3/providers/real.ts');client.fetcher=async()=>Response.json({authenticated:false});
 const {renderRealGoalPlanning}=await load('/src/v3-domain/goal-planning.js');
 const {configureDomainService}=await load('/src/v3/proto/services.js'),{_debug}=await load('/src/v3/proto/store.js');
 const state={revision:1,facts_revision:'facts',goals:[],dependencies:[],source_reviews:[],auto_planning_enabled:false,ordering_mode:'automatic',max_active_goals:3,active_count:0};
 configureDomainService('life',{goalSet:async()=>{reads++;return state;},planningHistory:async()=>[]});
 const route={fid:'A',path:'/c/A/life/planning',query:new URLSearchParams(),feature:{title:'目标规划',params:{},render:'renderRealGoalPlanning',load:async()=> (host,ctx)=>{_debug.data.session={actorId:'header-owner'};_debug.data.characters.A={id:'A',status:'active'};return renderRealGoalPlanning(host,ctx);}}};
 const v=await mountReact(vite,Page,{workspace:w,props:{route}});
 try{assert.equal(v.host.querySelector('h1').textContent,'目标规划');assert.equal(v.host.querySelector('nav a').getAttribute('href'),'#/c/A/life/goals');const before=reads;assert.ok(v.host.querySelector('[data-policy]'));const form=v.host.querySelector('[data-policy]');await v.input(form.elements.reason,'保留未保存的原因');await v.click(v.host.querySelector('[aria-label="重新读取目标规划"]'));assert.equal(reads,before+1);assert.equal(v.host.querySelector('[data-policy]').elements.reason.value,'保留未保存的原因');}finally{await v.dispose();}
});

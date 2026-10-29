import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
import {feedbackDOM} from './feedback-dom-audit.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const version=process.env.AUDIT_VERSION||'after',phase=process.env.AUDIT_PHASE||'feedback1',out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/feedback-read-'+version+'-'+phase);
await mkdir(out,{recursive:true});
const scenarios=[
 {name:'now',route:'/c/linwan/life/now',pattern:/\/api\/fluctlights\/linwan\/schedules\?/,empty:v=>({...v,days:v.days.map(d=>({...d,schedule:{...d.schedule,items:[]}}))})},
 {name:'wardrobe',route:'/c/linwan/life/wardrobe',pattern:/\/api\/fluctlights\/linwan\/(?:wardrobe|inventory)(?:\?|$)/,empty:v=>({...v,items:[],worn_items:[],used_items:[]})},
 {name:'profile',route:'/c/linwan/profile',pattern:/\/api\/fluctlights\/linwan\/detail$/,empty:v=>({...v,identity:{name:'林晚',timezone:'Asia/Shanghai'},personality:{},behavioral_policy:{},life_profile:{},core_persona:{identity:{name:'林晚',timezone:'Asia/Shanghai'},personality_system:{mode:'single',profiles:[]}},relationships:[]})},
 {name:'settings',route:'/settings',pattern:/\/api\/configuration-catalog$/,empty:v=>({...v,endpoints:[],bindings:[],effective_values:{},stored_values:{},versions:{}})},
 {name:'chat',route:'/c/linwan/chat',pattern:/\/api\/(?:fluctlights\/linwan\/conversation$|conversations\/parity-conversation-linwan\/messages(?:\?|$))/,empty:v=>({...v,messages:[]})},
 {name:'schedule',route:'/c/linwan/life/schedule',pattern:/\/api\/fluctlights\/linwan\/schedules\?/,empty:v=>({...v,days:v.days.map(d=>({...d,schedule:{...d.schedule,items:[]}}))})},
 {name:'goals',route:'/c/linwan/life/goals',pattern:/\/api\/fluctlights\/linwan\/goals(?:\?|$)/,empty:v=>({...v,items:[]})},
 {name:'diary',route:'/c/linwan/life/diary',pattern:/\/api\/fluctlights\/linwan\/diary\?/,empty:v=>({...v,items:[]})},
 {name:'memory',route:'/c/linwan/growth',pattern:/\/api\/fluctlights\/linwan\/memories\?/,empty:v=>({...v,items:[]})},
 {name:'world',route:'/c/linwan/world',pattern:/\/api\/fluctlights\/linwan\/world\/catalog$/,empty:v=>({...v,items:[],active_binding:null})},
 {name:'people',route:'/c/linwan/people',pattern:/\/api\/fluctlights\/linwan\/npcs(?:\?|$)/,empty:v=>({...v,items:[]})},
 {name:'feed',route:'/feed',pattern:/\/api\/moment-feed(?:\?|$)/,empty:v=>({...v,items:[]})},
 {name:'media',route:'/c/linwan/media',pattern:/\/api\/media-library\?/,empty:v=>({...v,items:[]})},
 {name:'studio',route:'/studio',pattern:/\/api\/studio\/resources\?/,empty:v=>({...v,items:[]})},
 {name:'inbox',route:'/inbox',pattern:/\/api\/notifications(?:\?|$)/,empty:v=>({...v,items:[]})},
];
const selected=process.env.AUDIT_CASES?.split(','),states=process.env.AUDIT_STATES?.split(',')||['normal','empty','loading','failure'];
const server=await startOwnedFixture({port:Number(process.env.FEEDBACK_READ_PORT||4475),env:{FLUCTLIGHT_UI_FIXTURE_DEMO:'normal',...(version==='before'?{FLUCTLIGHT_UI_FIXTURE_DIST:'/private/tmp/fluctlight-demo-audit-20261028/dist-before-feedback-20261029'}:{})}});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const scenario of scenarios){if(selected&&!selected.includes(scenario.name))continue;for(const state of states){
 const context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(8000);
 const record={page:scenario.name,state,size,version,boundary:'production dist and controlled HTTP matching Demo content; read states only, no real backend/model certification',errors:[],checks:[],requests:[],feedback:[],screenshots:[]};
 let release,unblock,sawRead,onRead,calls=0,failed=true;const observed=new Promise(ok=>sawRead=ok),held=new Promise(ok=>unblock=ok);
 page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method()});});
 await page.route(scenario.pattern,async route=>{
  // Current and historical goals have separate readers/retries. This case
  // injects the current partition only; never ask one retry to repair both.
  if(scenario.name==='goals'&&state==='failure'&&new URL(route.request().url()).searchParams.get('history')==='true')return route.continue();
  calls++;sawRead();onRead?.();if(state==='loading'&&!release)await held;
  if(state==='failure'&&failed)return route.fulfill({status:503,json:{code:'request_failed',message:'本次读取暂不可用',request_id:'diagnostic-feedback-request-only'}});
  if(state==='empty'){const response=await route.fetch();return route.fulfill({response,json:scenario.empty(await response.json())});}
  return route.continue();
 });
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
 const shot=async label=>{
  await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});
  const nodes=await feedbackDOM(page);record.feedback.push({label,nodes});const file=resolve(out,scenario.name+'-'+state+'-'+size+'-'+label+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);
  check(label+': no document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  if(version==='after')for(const node of nodes.filter(n=>n.kind)){
   check(label+': explicit '+node.kind+' has accessible semantics',node.kind==='error'?node.role==='alert'||!!node.className?.includes('field-error'):['progress','success','warning'].includes(node.kind)?node.role==='status':['note','status'].includes(node.role));
  }
  return nodes;
 };
 try{
  await page.goto(server.base+'/#'+scenario.route,{waitUntil:'domcontentloaded'});
  if(state==='loading'){
   await observed;await (version==='before'&&scenario.name==='chat'?page.getByText('正在读取对话…',{exact:true}):page.getByRole('status').filter({hasText:/正在读取|正在准备|加载中/})).first().waitFor();const nodes=await shot('waiting');if(version==='after')check('held read is progress without error severity',!nodes.some(n=>n.kind==='error'));
   release=true;unblock();await page.waitForLoadState('networkidle');await page.locator('h1').first().waitFor();await shot('resolved');check('resolving a read does not issue a command',record.requests.every(r=>r.method==='GET'));
  }else{
   await page.waitForLoadState('networkidle');if(state!=='failure')await page.locator('h1').first().waitFor();check('actual domain read was intercepted',calls>0);
   if(state==='failure'){
    const retry=page.getByRole('button',{name:/重新读取|重试|重新加载/}).first();await retry.waitFor();const nodes=await shot('failed');if(version==='after')check('read failure has explicit error or retained warning',nodes.some(n=>['error','warning'].includes(n.kind)));
    check('technical request identity is not ordinary copy',!(await page.locator('body').innerText()).includes('diagnostic-feedback-request-only'));
    const before=calls,nextRead=new Promise(ok=>onRead=ok);failed=false;await retry.click();let timer;try{await Promise.race([nextRead,new Promise((_,no)=>timer=setTimeout(()=>no(Error('retry did not reread the selected domain')),8000))]);}finally{clearTimeout(timer);}await page.waitForLoadState('networkidle');check('retry reaches original domain endpoint',calls>before);if(version==='after')await page.waitForFunction(()=>![...document.querySelectorAll('[data-feedback-kind=error],[data-feedback-kind=warning]')].some(e=>e.getClientRects().length&&/暂未读全|暂时无法连接|未能读取|读取失败/.test(e.innerText)));const recovered=await shot('recovered');if(version==='after')check('authoritative recovery clears old read failure',!recovered.some(n=>['error','warning'].includes(n.kind)&&/暂未读全|暂时无法连接|未能读取|读取失败/.test(n.text)));
    check('read recovery never writes a command',record.requests.every(r=>r.method==='GET'));
   }else await shot(state);
  }
  check('no browser exception',record.errors.length===0);record.status=version==='before'?'captured':'passed';
 }catch(e){record.status='failed';record.error=e.stack;record.text=await page.locator('body').innerText().catch(()=>null);await page.screenshot({path:resolve(out,scenario.name+'-'+state+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
 finally{release=true;unblock();await context.close();}
 results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(scenario.name,state,size,record.status,record.error||record.checks.length);
}}}finally{await browser.close();await server.stop();}
if(results.some(r=>r.status==='failed'))process.exitCode=1;

import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/states');await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
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
const selected=process.env.AUDIT_CASES?.split(',');
try{for(const scenario of scenarios){if(selected&&!selected.includes(scenario.name))continue;for(const state of ['empty','loading','failure']){
 const context=await browser.newContext({viewport:{width:390,height:844},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(7000);
 const record={page:scenario.name,state,viewport:{width:390,height:844},kind:'production dist with intercepted test HTTP read state; formal error/loading state has no simulated-success claim',errors:[],checks:[]};page.on('pageerror',e=>record.errors.push(e.message));
 let release,unblock,sawRead,recoveryRead,failed=true,calls=0;record.requests=[];
 const observed=new Promise(r=>sawRead=r);
 const held=new Promise(r=>unblock=r);
 await page.route(scenario.pattern,async route=>{calls++;record.requests.push({url:route.request().url(),failed});sawRead();recoveryRead?.();if(state==='loading'&&!release)await held;if(state==='failure'&&failed)return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:'request_failed',message:'本次界面专项读取失败'})});if(state==='empty'){const response=await route.fetch();return route.fulfill({response,json:scenario.empty(await response.json())});}return route.continue();});
 const shot=async label=>{const path=resolve(root,scenario.name+'-'+state+(label?'-'+label:'')+'.png');await page.screenshot({path});(record.screenshots||=[]).push(path);};
 try{
  await page.goto('http://127.0.0.1:4381/#'+scenario.route,{waitUntil:'domcontentloaded'});
  if(state==='loading'){
   await observed;await page.locator('h1').first().waitFor();await page.getByText(/正在读取|正在准备/).first().waitFor();await shot('waiting');release=true;unblock();await page.waitForLoadState('networkidle');await page.locator('h1').waitFor();record.checks.push('loading is visible while the actual HTTP read is held','held read completes without a replacement command');await shot('resolved');
  }else{
   await page.waitForLoadState('networkidle');if(state!=='failure')await page.locator('h1').first().waitFor();
   if(state==='failure'){
    const retry=page.getByRole('button',{name:/重新读取|重试|重新加载/}).first();await retry.waitFor();await shot('error');failed=false;const before=calls;const nextRead=new Promise(resolve=>recoveryRead=resolve);await retry.click();await Promise.race([nextRead,new Promise((_,reject)=>setTimeout(()=>reject(Error('recovery did not reach selected endpoint')),7000))]);await page.waitForLoadState('networkidle');if(scenario.name==='chat')await page.getByText('给你看看窗边这个小角落。',{exact:false}).waitFor();assert.ok(calls>before,'retry must reread the selected endpoint');record.checks.push('read failure exposes a recovery action','recovery reads the original domain endpoint');await shot('recovered');
   }else{assert.ok(calls>0,'empty scene must use its actual API read');record.checks.push('empty response passes through the production page and data mapper');await shot('empty');}
  }
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'narrow state must not overflow');assert.equal(record.errors.length,0);record.checks.push('no document horizontal overflow','no uncaught browser exception');record.status='passed';
 }catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}finally{release=true;unblock();record.calls=calls;results.push(record);console.log(scenario.name,state,record.status,record.error||'');await writeFile(resolve(root,'read-states'+(selected?'-'+selected.join('-'):'')+'.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

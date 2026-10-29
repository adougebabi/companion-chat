import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/feed-state-edges-'+(process.env.AUDIT_PHASE||'pass121'));
await mkdir(out,{recursive:true});const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of ['suggestion-states','suggestion-readback','npc-states','comment-read-failure']){
 const server=await startOwnedFixture({port:Number(process.env.FEED_EDGES_PORT||4469),env:{FLUCTLIGHT_UI_FIXTURE_FEED:'normal'}});let context,page;
 const record={name,size,width,height,boundary:'production dist / controlled HTTP request outcomes; no live model publication or encounter proof',checks:[],requests:[],errors:[],screenshots:[]};
 try{
  context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(10000);
  page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
  const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);},d=()=>page.getByRole('dialog').last(),writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
  const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,name+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
  const go=()=>page.goto(server.base+'/#/feed',{waitUntil:'networkidle'});
  let openingPost=false;const openPost=async()=>{const card=page.getByTestId('feed-post-post-one');for(let n=0;n<6&&!await card.count();n++)await page.getByRole('button',{name:'加载更多动态',exact:true}).click();openingPost=true;await card.getByRole('button',{name:'查看详情',exact:true}).click();await d().waitFor();};
  const resume=async()=>{if(size==='mobile')await page.getByRole('button',{name:'你们的分享建议与原请求',exact:true}).click();await page.getByRole('button',{name:'继续查看上次分享建议',exact:true}).first().click();};
  let admission,status='processing',revision=2,failedRead=false;
  if(name.startsWith('suggestion')){
   await page.route('**/api/fluctlights/fluctlight-fixture-one/moment-suggestions',async route=>{if(route.request().method()==='POST'){const response=await route.fetch();admission=await response.json();await route.fulfill({response});}else await route.continue();});
   await page.route(/\/api\/(?:fluctlights\/[^/]+\/)?owner-requests\/suggestion-[^/?]+$/,async route=>{
    if(!admission||!new URL(route.request().url()).pathname.endsWith('/'+admission.request.id)){await route.continue();return;}
    if(name==='suggestion-readback'&&!failedRead){failedRead=true;await route.fulfill({status:503,json:{code:'request_failed',message:'受控原建议首次回读失败'}});return;}
    // Never fetch repeatedly: the fixture's third GET creates a post, while
    // these cases deliberately exercise non-published request outcomes.
    await route.fulfill({json:{...admission.request,status,revision,decision_reason:status==='rejected'?'暂时不想分享，原建议仍保留。':status==='failed'?'本次分享判断没有完成，原建议已保留。':'',result:{}}});
   });
  }
  if(name==='comment-read-failure')await page.route('**/api/moment-feed/post-one/comments*',async route=>{if(route.request().method()==='GET'&&openingPost&&!failedRead){failedRead=true;await route.fulfill({status:503,json:{code:'request_failed',message:'受控评论首次读取失败'}});}else await route.continue();});
  let npcStatus='suppressed';
  if(name==='npc-states')await page.route('**/api/moment-feed/post-one/npc-interactions*',async route=>{const response=await route.fetch(),value=await response.json();await route.fulfill({response,json:{...value,items:value.items.map(r=>({...r,status:npcStatus,response:{reason:npcStatus==='suppressed'?'原世界来源已变化，本次未产生互动。':'受控原互动结果已提交，来源继续保留。'},revision:5}))}});});
  await go();
  if(name.startsWith('suggestion')){
   await page.getByRole('button',{name:'邀请分享近况',exact:true}).click();await d().getByRole('combobox',{name:'建议哪位角色',exact:true}).click();await page.getByRole('option').filter({hasText:'林岚'}).click();const text='原分享建议 '+name+' '+size;await d().getByRole('textbox',{name:'分享建议',exact:true}).fill(text);await d().getByRole('button',{name:'提出分享建议',exact:true}).click();
   if(name==='suggestion-readback'){
    await d().getByRole('button',{name:'只读恢复页面结果',exact:true}).waitFor();await shot('accepted-read-failure');await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});await page.reload({waitUntil:'networkidle'});await page.getByRole('button',{name:'继续核对分享建议',exact:true}).click();await d().getByRole('button',{name:'只读恢复页面结果',exact:true}).click();
   }
   await d().getByText('正在考虑',{exact:true}).waitFor();check('processing is not an actual publication',await d().getByRole('button',{name:'查看角色实际发布',exact:true}).count()===0);await shot('processing');
   if(name==='suggestion-states'){
    status='rejected';revision=3;await d().getByRole('button',{name:'刷新原请求',exact:true}).click();await d().getByText('暂未采纳',{exact:true}).waitFor();await d().getByText('暂时不想分享，原建议仍保留。',{exact:true}).first().waitFor();await shot('rejected');
   }
   await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});if(name==='suggestion-states'){status='failed';revision=4;}await page.reload({waitUntil:'networkidle'});await resume();
   if(name==='suggestion-states'){await d().getByText('处理失败',{exact:true}).waitFor();await d().getByRole('button',{name:'重新和她商量',exact:true}).waitFor();await shot('failed-original');}else await d().getByText('正在考虑',{exact:true}).waitFor();
   await d().getByRole('button',{name:'原请求与运行记录',exact:true}).click();check('refresh resumes same request and original text',(await d().innerText()).includes(admission.request.id)&&(await d().innerText()).includes(text));check('recovery and state refresh never resubmit or auto-publish',writes().length===1&&writes()[0].path.endsWith('/moment-suggestions'));await shot('recovered');
  }else if(name==='npc-states'){
   await openPost();await d().getByRole('button',{name:'世界人物互动与实际来源',exact:true}).click();await d().getByText('来源变化，未产生互动',{exact:true}).waitFor();await d().getByRole('button',{name:'原结识、来源与互动记录',exact:true}).click();await d().getByText('"copresent": false',{exact:false}).waitFor();check('suppressed source retains original encounter without copresence',(await d().innerText()).includes('actual-encounter'));await shot('suppressed');await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});npcStatus='completed';await openPost();await d().getByRole('button',{name:'世界人物互动与实际来源',exact:true}).click();await d().getByText('实际互动已提交',{exact:true}).waitFor();check('completed interaction has no retry command',await d().getByRole('button',{name:'重试本次互动',exact:true}).count()===0);await shot('completed');check('reading outcomes makes no NPC or relation write',writes().length===0);
  }else{
   await openPost();await d().getByRole('button',{name:'重新读取评论',exact:true}).waitFor();check('comment read failure preserves post and is not a confirmed empty list',await d().getByText('还没有回应，你可以留下第一句话。',{exact:true}).count()===0);await shot('failed');await d().getByRole('button',{name:'重新读取评论',exact:true}).click();for(const text of ['这一句读起来很温柔。','我也想读读这本书。'])await d().getByText(text,{exact:true}).waitFor();await shot('recovered');check('comment retry only reads original post',writes().length===0&&record.requests.filter(r=>r.path.endsWith('/comments')).every(r=>r.path==='/api/moment-feed/post-one/comments'));
  }
  check('no browser exception',record.errors.length===0);record.status='passed';
 }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
 finally{try{await context?.close();}finally{await server.stop();}}
 results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

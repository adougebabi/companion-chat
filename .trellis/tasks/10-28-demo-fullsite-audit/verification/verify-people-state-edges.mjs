import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/people-state-edges-'+(process.env.AUDIT_PHASE||'pass119'));
await mkdir(out,{recursive:true});const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of ['incomplete','foreign-world','foreign-place','read-only','about-conflict']){
 const server=await startOwnedFixture({port:Number(process.env.PEOPLE_EDGES_PORT||4467),env:{FLUCTLIGHT_UI_FIXTURE_PEOPLE:'normal',FLUCTLIGHT_UI_FIXTURE_PROFILE:'normal'}});let context,page,release;
 const record={name,size,width,height,boundary:'actual production components / controlled People and Profile HTTP; no live cognition or identity writes',checks:[],requests:[],errors:[],screenshots:[]};
 try{
  context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(10000);
  page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
  const api='/api/fluctlights/fluctlight-fixture-one',check=(label,value)=>{assert.ok(value,label);record.checks.push(label);},writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
  const get=async path=>{const r=await context.request.get(server.base+path);assert.equal(r.status(),200,path);return r.json();};
  const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,name+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
  let invalid=true;
  if(name==='incomplete')await page.route('**'+api+'/npcs',async route=>{const response=await route.fetch(),value=await response.json();await route.fulfill({response,json:invalid?{...value,next_cursor:'unread-page'}:value});});
  if(name.startsWith('foreign-'))await page.route('**'+api+'/world',async route=>{const response=await route.fetch(),value=await response.json();if(invalid){if(name==='foreign-world')value.world.fluctlight_id='foreign-role';else value.places[0].world_id='foreign-world';}await route.fulfill({response,json:value});});
  if(name==='read-only')await page.route('**'+api+'/detail',async route=>{const response=await route.fetch(),value=await response.json();await route.fulfill({response,json:{...value,status:'retired'}});});
  await page.goto(server.base+'/#/c/fluctlight-fixture-one/people',{waitUntil:'networkidle'});
  if(['incomplete','foreign-world','foreign-place'].includes(name)){
   const reason=name==='incomplete'?'人物目录尚未完整返回。':name==='foreign-world'?'当前世界不属于此角色。':'人物、地点与当前世界范围不匹配，请重新读取。';
   await page.getByRole('alert').filter({hasText:reason}).waitFor();check('invalid successful HTTP projection is not confirmed empty',await page.getByRole('button',{name:'查看人物 苏禾',exact:true}).count()===0);await shot('scope-failure');invalid=false;await page.getByRole('button',{name:'重新读取世界人物',exact:true}).click();await page.getByRole('button',{name:'查看人物 苏禾',exact:true}).waitFor();await shot('recovered');check('scope retry never mutates a person',writes().length===0);
  }else if(name==='read-only'){
   await page.getByRole('button',{name:'查看人物 苏禾',exact:true}).waitFor();check('retired role disables person creation',await page.getByRole('button',{name:'新增世界人物',exact:true}).isDisabled());await page.getByRole('button',{name:'查看人物 苏禾',exact:true}).click();const d=page.getByRole('dialog').last();check('readable detail disables editing and management',await d.getByRole('button',{name:'编辑人物资料',exact:true}).isDisabled()&&await d.getByRole('button',{name:'人物管理',exact:true}).isDisabled());await shot('detail-read-only');await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});await page.getByRole('tab',{name:'关于你',exact:true}).click();await page.getByRole('button',{name:'编辑资料',exact:true}).waitFor();check('retired role also disables authored background edits',await page.getByRole('button',{name:'编辑资料',exact:true}).isDisabled());await shot('about-read-only');check('read-only inspection sends no mutation',writes().length===0);
  }else{
   await page.getByRole('tab',{name:'关于你',exact:true}).click();await page.getByRole('button',{name:'编辑资料',exact:true}).click();const inline=page.locator('[data-slot="workspace-command-inline"]'),draft='事实冲突后保留的自填称呼';await inline.getByRole('textbox',{name:'称呼',exact:true}).fill(draft);await inline.getByRole('button',{name:'保存资料',exact:true}).click();await inline.getByRole('alert').filter({hasText:'请填写修改说明'}).waitFor();check('blank reason prevents authored write',writes().length===0);await shot('invalid');await inline.getByRole('textbox',{name:'修改说明',exact:true}).fill('核对最新事实基线后保留这次自填修改');
   const before=await get(api+'/detail');const external=await context.request.put(server.base+api+'/actor-user-background',{data:{background:{occupation:'外部已确认的新职业'},operation:'change',reason:'外部确认的测试变化',expectedCurrentFactsRevision:before.current_facts_revision,idempotencyKey:'external-people-baseline'}});assert.equal(external.status(),200);
   const gate=new Promise(ok=>{release=ok;});let held=true;
   await page.route('**'+api+'/actor-user-background',async route=>{if(held&&route.request().method()==='PUT'){held=false;await gate;}await route.continue();});
   const sent=page.waitForRequest(r=>new URL(r.url()).pathname===api+'/actor-user-background'&&r.method()==='PUT');await inline.getByRole('button',{name:'保存资料',exact:true}).click();await sent;await inline.getByRole('button',{name:'正在处理…',exact:true}).waitFor();check('authored save busy blocks duplicate confirmation',await inline.getByRole('button',{name:'正在处理…',exact:true}).isDisabled());await shot('saving');release();release=null;
   await inline.getByRole('alert').waitFor();await inline.getByRole('button',{name:'读取最新版本并保留输入',exact:true}).waitFor();check('conflict retains same unsaved field',await inline.getByRole('textbox',{name:'称呼',exact:true}).inputValue()===draft);await shot('conflict');
   await inline.getByRole('button',{name:'读取最新版本并保留输入',exact:true}).click();await inline.getByRole('button',{name:'已核对，采用最新基线',exact:true}).click();check('explicit rebase preserves input and makes no new PUT',await inline.getByRole('textbox',{name:'称呼',exact:true}).inputValue()===draft&&writes().length===1);await shot('rebased');await inline.getByRole('button',{name:'保存资料',exact:true}).click();await page.getByText('你的资料已保存并读回，自填来源与已有认识分别保留。',{exact:true}).waitFor();
   const after=await get(api+'/detail');check('rebased write uses latest facts and keeps external unrelated field',after.actor_user.background.name===draft&&after.actor_user.background.occupation==='外部已确认的新职业'&&writes().length===2&&writes()[0].body.expectedCurrentFactsRevision!==writes()[1].body.expectedCurrentFactsRevision);await shot('saved');
  }
  check('no browser exception',record.errors.length===0);record.status='passed';
 }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
 finally{release?.();try{await context?.close();}finally{await server.stop();}}
 results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

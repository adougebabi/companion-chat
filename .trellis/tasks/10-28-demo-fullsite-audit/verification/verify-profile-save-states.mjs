import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/profile-save-states-'+(process.env.AUDIT_PHASE||'pass110'));
await mkdir(out,{recursive:true});const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of ['validation-busy','conflict-rebase']){
  const server=await startOwnedFixture({port:Number(process.env.PROFILE_STATES_PORT||4459),env:{FLUCTLIGHT_UI_FIXTURE_PROFILE:'normal'}});
  let context,page,release;
  const record={name,size,width,height,boundary:'production dist / fresh controlled profile HTTP; no real identity mutation',checks:[],requests:[],errors:[],screenshots:[]};
  try{
    context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(10000);
    page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))record.requests.push({path:new URL(r.url()).pathname,method:r.method(),body:r.postDataJSON()});});
    const api='/api/fluctlights/fluctlight-fixture-one',dialog=()=>page.getByRole('dialog').last(),writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
    const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
    const get=async()=>{const r=await context.request.get(server.base+api+'/detail');assert.equal(r.status(),200);return r.json();};
    const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));if(await dialog().count()){const b=await dialog().boundingBox();if(b)check(state+': bounded dialog',b.x>=0&&b.y>=0&&b.x+b.width<=width+.5&&b.y+b.height<=height+.5);}const file=resolve(out,name+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
    await page.goto(server.base+'/#/c/fluctlight-fixture-one/profile',{waitUntil:'networkidle'});await page.getByRole('button',{name:'编辑资料',exact:true}).click();
    const field=()=>dialog().getByRole('textbox',{name:'名字',exact:true});
    if(name==='validation-busy'){
      await field().fill('');await dialog().getByRole('button',{name:'保存资料',exact:true}).click();await dialog().getByRole('alert').filter({hasText:'请填写角色名字'}).waitFor();
      check('empty name never submits and gets focus',writes().length===0&&await field().evaluate(e=>e===document.activeElement));await shot('invalid');
      const gate=new Promise(ok=>{release=ok;});let arrived;const received=new Promise(ok=>{arrived=ok;});
      await page.route('**'+api+'/foundation-revisions',async route=>{if(route.request().method()==='POST'){arrived();await gate;}await route.continue();});
      await field().fill('资料保存等待验收');await dialog().getByRole('button',{name:'保存资料',exact:true}).click();await received;
      check('busy blocks duplicate save and cancel',await page.getByTestId('detail-save').isDisabled()&&await dialog().getByRole('button',{name:'取消',exact:true}).isDisabled());
      await page.keyboard.press('Escape');check('busy Escape retains editor',await dialog().count()===1);await shot('saving');release();release=null;
      await page.getByRole('dialog').waitFor({state:'detached'});check('one save creates and accepts exactly one proposal',writes().length===2&&writes()[0].path.endsWith('/foundation-revisions')&&writes()[1].path.endsWith('/accept'));
      check('confirmed detail is authoritative',(await get()).identity.name==='资料保存等待验收');await page.getByRole('button',{name:'编辑资料',exact:true}).click();check('reopen reads saved name',await field().inputValue()==='资料保存等待验收');await shot('saved');
    }else{
      const before=await get(),draft='版本冲突之后保留的名字';await field().fill(draft);
      // Explicit external setup advances the actual controlled baseline; it is
      // separate from page writes and has no Core/model side effects.
      const external=await context.request.post(server.base+api+'/foundation-revisions/fluctlight-fixture-one-foundation-proposed/accept',{data:{expectedRevision:before.current_revision,reason:'外部已确认的验收基线变化'}});assert.equal(external.status(),200);
      const advanced=await get();check('external setup truly advances revision',advanced.current_revision===before.current_revision+1);
      await dialog().getByRole('button',{name:'保存资料',exact:true}).click();await dialog().getByRole('button',{name:'读取最新资料，保留当前编辑',exact:true}).waitFor();
      check('stale save is rejected before proposal acceptance',writes().length===1&&writes()[0].body.expectedRevision===before.current_revision);
      check('conflict preserves unsaved field',await field().inputValue()===draft);await shot('conflict');
      await dialog().getByRole('button',{name:'读取最新资料，保留当前编辑',exact:true}).click();await dialog().getByRole('button',{name:'读取最新资料，保留当前编辑',exact:true}).waitFor({state:'detached'});
      check('explicit rebase is GET-only and preserves field',writes().length===1&&await field().inputValue()===draft);await shot('rebased');
      await dialog().getByRole('button',{name:'保存资料',exact:true}).click();await page.getByRole('dialog').waitFor({state:'detached'});
      const after=await get();check('rebased save uses new revision and independent new key',writes().length===3&&writes()[1].body.expectedRevision===advanced.current_revision&&writes()[0].body.idempotencyKey!==writes()[1].body.idempotencyKey);
      check('saved edited field retains externally confirmed other fields',after.identity.name===draft&&after.life_profile.life_habits===advanced.life_profile.life_habits&&after.current_revision===advanced.current_revision+1);await shot('saved');
    }
    check('no browser exception',record.errors.length===0);record.status='passed';
  }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
  finally{release?.();try{await context?.close();}finally{await server.stop();}}
  results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

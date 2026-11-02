// Run after the production build, with Playwright Core installed separately:
// PLAYWRIGHT_MODULE=/absolute/path/to/playwright-core/index.mjs node apps/web/scripts/verify-r9-controls.mjs
// Uses only a process-owned loopback in-memory fixture; no Core or database.
import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import net from 'node:net';
import {creationDraft} from '../src/v3/creation-draft.mjs';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright-core');
const repo=fileURLToPath(new URL('../../../',import.meta.url));
const output='/tmp/r9-shots',port=Number(process.env.R9_FIXTURE_PORT||4299),base=`http://127.0.0.1:${port}`;
assert.ok(Number.isSafeInteger(port)&&port>=1024&&port<=65535);
await mkdir(output,{recursive:true});
const probe=net.createServer();
await new Promise((ok,no)=>{probe.once('error',no);probe.listen(port,'127.0.0.1',ok);});await new Promise(ok=>probe.close(ok));
const fixture=spawn(process.execPath,['apps/web/test/fixtures/ui-quality-server.mjs'],{cwd:repo,env:{...Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('FLUCTLIGHT_UI_FIXTURE_'))),FLUCTLIGHT_UI_FIXTURE_PORT:String(port),FLUCTLIGHT_UI_FIXTURE_DIST:resolve(repo,'apps/web/dist'),FLUCTLIGHT_UI_FIXTURE_STUDIO:'normal',FLUCTLIGHT_UI_FIXTURE_CREATION:'normal',FLUCTLIGHT_UI_FIXTURE_MEDIA:'normal',FLUCTLIGHT_UI_FIXTURE_FEED:'normal'},stdio:['ignore','pipe','pipe']});
let browser,pageForFailure;
const results=[],errors=[];
try{
  await new Promise((ok,no)=>{let out='',err='';const timer=setTimeout(()=>no(Error('fixture timeout: '+err)),10000);const fail=e=>{clearTimeout(timer);no(e);};fixture.once('error',fail);fixture.once('exit',code=>fail(Error('fixture exited '+code+': '+err)));fixture.stderr.on('data',data=>err+=data);fixture.stdout.on('data',data=>{out+=data;if(out.includes('Isolated UI fixture: '+base)){clearTimeout(timer);ok();}});});
  browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
  for(const [viewport,width,height] of [['desktop',1440,1000],['mobile',390,844]]){
    const context=await browser.newContext({viewport:{width,height},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
    const page=await context.newPage();pageForFailure=page;page.setDefaultTimeout(10000);let emptyHome=true;
    page.on('pageerror',error=>errors.push({viewport,message:error.message}));
    await page.route('**/*',async route=>{
      const url=new URL(route.request().url());
      if(url.origin!==base&&!['data:','blob:'].includes(url.protocol)){errors.push({viewport,external:url.origin});return route.abort();}
      if(emptyHome&&url.pathname==='/api/fluctlights'&&route.request().method()==='GET')return route.fulfill({json:[]});
      if(emptyHome&&url.pathname==='/api/configuration-catalog'){
        const response=await route.fetch(),body=await response.json();return route.fulfill({response,json:{...body,endpoints:[],bindings:[]}});
      }
      return route.continue();
    });
    const draft=creationDraft({owner:'human-fixture-owner',storage:{getItem:()=>null}}).read();
    await context.addInitScript(value=>{if(!localStorage.getItem('fluctlight-creation:human-fixture-owner:draft'))localStorage.setItem('fluctlight-creation:human-fixture-owner:draft',JSON.stringify(value));},draft);
    const open=async path=>{await page.goto(base+'/#/'+path,{waitUntil:'networkidle'});};
    const audit=async label=>{
      // Measure final geometry after Radix's finite entrance/exit animations.
      // Keep exact 36px assertions; an opening Dialog starts at scale(.95).
      await page.evaluate(async()=>{
        await Promise.all(document.getAnimations().filter(animation=>Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation=>animation.finished.catch(()=>{})));
        await new Promise(done=>requestAnimationFrame(()=>requestAnimationFrame(done)));
      });
      const data=await page.evaluate(()=>{
        const visible=e=>!!e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden';
        const measure=e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e),p=getComputedStyle(e,'::after');return {label:e.getAttribute('aria-label')||e.textContent.trim(),size:e.dataset.size,slot:e.dataset.slot,width:r.width,height:r.height,pseudo:{content:p.content,width:p.width,height:p.height,background:p.backgroundColor},padding:[s.paddingTop,s.paddingRight,s.paddingBottom,s.paddingLeft],position:s.position};};
        return {route:location.hash,documentWidth:document.documentElement.scrollWidth,icons:[...document.querySelectorAll('[data-size="icon"]:not([data-day])')].filter(visible).map(measure),targets:[...document.querySelectorAll('[role="radio"],[role="checkbox"],.tag-filter>button')].filter(visible).map(measure)};
      });
      assert.ok(data.icons.length>0,label+' contains actual icon Buttons');
      for(const icon of data.icons){assert.equal(icon.width,36,label+': '+JSON.stringify(icon));assert.equal(icon.height,36,label+': '+JSON.stringify(icon));assert.deepEqual(icon.padding,['0px','0px','0px','0px'],label+': '+JSON.stringify(icon));}
      assert.ok(data.documentWidth<=width,label+' must not overflow horizontally: '+data.documentWidth);
      if(viewport==='mobile')for(const target of [...data.icons,...data.targets]){
        assert.equal(target.pseudo.content,'""',label+': '+target.label+' needs a generated hit box');
        assert.ok(parseFloat(target.pseudo.width)>=44,label+': '+JSON.stringify(target));assert.ok(parseFloat(target.pseudo.height)>=44,label+': '+JSON.stringify(target));assert.equal(target.pseudo.background,'rgba(0, 0, 0, 0)');
      }
      results.push({viewport,label,...data});await page.screenshot({path:output+'/'+viewport+'-'+label+'.png',fullPage:true,animations:'disabled'});
    };
    const tabTo=async locator=>{
      await locator.scrollIntoViewIfNeeded();
      for(let i=0;i<100;i++){await page.keyboard.press('Tab');if(await locator.evaluate(e=>e===document.activeElement))return;}
      assert.fail('Tab never reached '+await locator.textContent());
    };
    await open('library');
    for(const [name,label] of [['继续创建','focus-continue'],['先连接模型','focus-connect']]){
      const link=page.getByRole('link',{name,exact:true});await tabTo(link);
      const focus=await link.evaluate(e=>({focusVisible:e.matches(':focus-visible'),shadow:getComputedStyle(e).boxShadow,outline:getComputedStyle(e).outlineWidth}));
      assert.equal(focus.focusVisible,true);assert.notEqual(focus.shadow,'none');assert.match(focus.shadow,/3px/);
      results.push({viewport,label,...focus});await page.screenshot({path:output+'/'+viewport+'-'+label+'.png',fullPage:true,animations:'disabled'});
      // Reintroducing the original unlayered decoration proves the root cause.
      const old=await page.addStyleTag({content:'[data-slot="button"][data-variant="outline"]{box-shadow:none}'});
      assert.equal(await link.evaluate(e=>getComputedStyle(e).boxShadow),'none');await old.evaluate(e=>e.remove());
      assert.notEqual(await link.evaluate(e=>getComputedStyle(e).boxShadow),'none');
    }
    await audit('home');
    if(viewport==='mobile'){
      const all=page.getByRole('button',{name:'全部',exact:true});await all.scrollIntoViewIfNeeded();
      const box=await all.boundingBox();assert.ok(box.height<32,'compact filter appearance is retained');
      assert.equal(await all.evaluate((e,{x,y})=>document.elementFromPoint(x,y)===e,{x:box.x+box.width/2,y:box.y-4}),true,'transparent filter extension participates in hit testing');
    }
    await open('library/create');await page.getByRole('tab',{name:'日常生活世界',exact:true}).click();const daily=page.getByRole('radio',{name:'使用日常生活世界',exact:true});await daily.waitFor();
    const circle=await daily.boundingBox();assert.equal(circle.width,16);assert.equal(circle.height,16);
    if(viewport==='mobile'){
      await daily.scrollIntoViewIfNeeded();const box=await daily.boundingBox(),x=box.x+box.width/2,y=box.y-6;
      assert.equal(await daily.evaluate((e,{x,y})=>document.elementFromPoint(x,y)===e,{x,y}),true,'transparent area outside the 16px radio is hit-testable');
      await page.mouse.click(x,y);assert.equal(await daily.getAttribute('aria-checked'),'true');
      const selected=await daily.boundingBox();
      const noHitPlane=await page.addStyleTag({content:'[data-slot="radio-group-item"]::after{content:none}'});
      const originalVisual=await daily.boundingBox();
      assert.equal(selected.width,originalVisual.width,'transparent radio target must not change selected visual width');
      assert.equal(selected.height,originalVisual.height,'transparent radio target must not change selected visual height');
      await noHitPlane.evaluate(e=>e.remove());
    }
    await audit('wizard');
    await open('worlds');await page.getByRole('checkbox',{name:'已归档资源',exact:true}).waitFor();
    if(viewport==='mobile'){
      const checkbox=page.getByRole('checkbox',{name:'已归档资源',exact:true});await checkbox.scrollIntoViewIfNeeded();const box=await checkbox.boundingBox();assert.equal(box.width,16);assert.equal(box.height,16);
      const x=box.x+box.width/2,y=box.y-6;
      assert.equal(await checkbox.evaluate((e,{x,y})=>document.elementFromPoint(x,y)===e,{x,y}),true,'transparent archive checkbox extension participates in hit testing');
      await page.mouse.click(x,y);assert.equal(await checkbox.getAttribute('aria-checked'),'true');
    }
    await audit('worlds');
    await open('studio');await page.getByRole('button',{name:'刷新创作资源',exact:true}).waitFor();await audit('studio');
    await open('inbox');await page.getByRole('button',{name:'重新读取通知',exact:true}).waitFor();await audit('inbox');
    await open('diag?view=tasks');await page.getByRole('button',{name:'重新读取运行任务',exact:true}).waitFor();await audit('diagnostics');
    await open('feed');await page.getByRole('button',{name:'刷新动态',exact:true}).waitFor();await audit('feed');
    await open('media');await page.getByRole('button',{name:'刷新媒体库',exact:true}).waitFor();await audit('media');
    emptyHome=false;await page.reload({waitUntil:'networkidle'});await audit('media-with-character');
    await page.getByRole('button',{name:'快速前往',exact:true}).click();await page.getByRole('dialog').waitFor();await audit('quick-jump');
    await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'hidden'});
    for(const [label,path] of [['now','life/now'],['schedule','life/schedule'],['goals','life/goals'],['wardrobe','life/wardrobe'],['diary','life/diary'],['memory','growth'],['runtime-world','world'],['people','people'],['profile','profile']]){
      await open('c/fluctlight-fixture-one/'+path);await audit(label);
    }
    await open('settings');await page.getByRole('heading',{name:'管理中心',exact:true}).waitFor();await audit('settings');
    if(viewport==='mobile'){
      await page.getByRole('button',{name:'打开导航',exact:true}).click();await page.getByRole('dialog').waitFor();await audit('mobile-navigation');
      await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'hidden'});
    }
    await context.close();
  }
  assert.deepEqual(errors,[]);
  await writeFile(output+'/measurements.json',JSON.stringify({boundary:'local production dist + owned in-memory fixture; no database or acceptance environment',results,errors},null,2)+'\n');
  console.log(JSON.stringify({passed:results.length,icons:results.reduce((n,r)=>n+(r.icons?.length||0),0),errors}));
}catch(error){
  if(pageForFailure&&!pageForFailure.isClosed()){await pageForFailure.screenshot({path:output+'/failure.png',fullPage:true});await writeFile(output+'/failure.txt',await pageForFailure.locator('body').innerText());}
  throw error;
}finally{
  await browser?.close();
  if(fixture.exitCode===null&&fixture.signalCode===null){const stopped=once(fixture,'exit');fixture.kill('SIGTERM');await stopped;}
}

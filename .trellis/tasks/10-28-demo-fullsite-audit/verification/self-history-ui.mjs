// Production dist + controlled in-memory HTTP. Run after the coordinated build.
// A fresh child-owned fixture is started for every case and viewport.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {spawn} from 'node:child_process';
import net from 'node:net';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {once} from 'node:events';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core');
const port=Number(process.env.SELF_HISTORY_PORT||4455),base=`http://127.0.0.1:${port}`;
assert.ok(Number.isSafeInteger(port)&&port>=1024&&port<=65535,'SELF_HISTORY_PORT must be an integer from 1024 to 65535');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/self-history-'+(process.env.AUDIT_PHASE||'pending'));
const fid='fluctlight-fixture-one',path='/api/fluctlights/'+fid+'/self-claims/'+fid+'-claim-one';
await mkdir(out,{recursive:true});
const results=[], selected=process.env.AUDIT_CASES?.split(',');
async function start(){
  // Refuse occupied ports: never attach to or terminate an unrelated server.
  const probe=net.createServer();
  await new Promise((ok,no)=>{probe.once('error',no);probe.listen(port,'127.0.0.1',ok);});
  await new Promise(ok=>probe.close(ok));
  const child=spawn(process.execPath,['apps/web/test/fixtures/ui-quality-server.mjs'],{cwd:process.cwd(),env:{...Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('FLUCTLIGHT_UI_FIXTURE_'))),FLUCTLIGHT_UI_FIXTURE_PORT:String(port),FLUCTLIGHT_UI_FIXTURE_GROWTH:'normal'},stdio:['ignore','pipe','pipe']});
  try {
    await new Promise((ok,no)=>{
      let stdout='',stderr='';
      const timer=setTimeout(()=>no(Error('fixture did not start: '+stdout+stderr)),10000);
      const fail=error=>{clearTimeout(timer);no(error);};
      child.once('error',fail);
      child.once('exit',(code,signal)=>fail(Error('fixture exited '+(signal||code)+': '+stdout+stderr)));
      child.stderr.on('data',value=>{stderr+=value;});
      child.stdout.on('data',value=>{
        stdout+=value;
        if(stdout.split('\n').includes('Isolated UI fixture: '+base)){
          clearTimeout(timer);ok();
        }
      });
    });
    return child;
  } catch(error){await stop(child);throw error;}
}
async function stop(child){
  if(child.exitCode!==null||child.signalCode!==null||!child.pid)return;
  const ended=once(child,'exit');
  child.kill('SIGTERM');
  await ended;
}
const browser=await chromium.launch({channel:'chrome',headless:true});
try {for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of ['normal','unknown','readback','adopt-refresh','status']){
  if(selected&&!selected.includes(name))continue;
  const child=await start();
  let context,page;
  try {
    context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
    page=await context.newPage();
  } catch(error){try{await context?.close();}finally{await stop(child);}throw error;}
  page.setDefaultTimeout(10000);
  const record={name,size,width,height,dpr:1,locale:'zh-CN',timezone:'Asia/Shanghai',boundary:'production dist / fresh child-owned controlled HTTP; no live Core/DB/source dependency certification',checks:[],requests:[],errors:[],screenshots:[]};
  const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
  const get=async suffix=>{const response=await context.request.get(base+path+(suffix||''));assert.equal(response.status(),200);return response.json();};
  const detail=()=>page.getByRole('dialog').last();
  const button=label=>detail().getByRole('button',{name:label,exact:true});
  const writes=()=>record.requests.filter(r=>r.method==='POST');
  const shot=async(label,{dialog=true}={})=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(label+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));if(dialog){const box=await detail().boundingBox();check(label+': bounded dialog',!!box&&box.x>=0&&box.y>=0&&box.x+box.width<=width+.5&&box.y+box.height<=height+.5);}const file=resolve(out,size+'-'+name+'-'+label+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
  const history=async expected=>{const trigger=button('之前的认识与历史恢复');if(await trigger.getAttribute('aria-expanded')!=='true')await trigger.click();for(const n of expected)await detail().locator('.lp-history-item').getByRole('heading',{name:new RegExp('^第 '+n+' 版')}).waitFor();};
  const enter=async(label,reason)=>{await button(label).click();await detail().getByRole('textbox',{name:'认识操作说明',exact:true}).fill(reason);};
  page.on('pageerror',e=>record.errors.push(e.message));
  page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))record.requests.push({method:r.method(),path:new URL(r.url()).pathname,body:r.postDataJSON()});});
  try {
    let changed=false,injected=false;
    if(['unknown','readback','status'].includes(name))await page.route('**'+path+'**',async route=>{
      const request=route.request(),url=new URL(request.url());
      if(request.method()==='POST'){
        const response=await route.fetch();changed=true;
        if(name==='unknown'&&!injected){injected=true;await route.fulfill({status:503,json:{code:'request_failed',message:'受控验收：已执行但回执丢失'}});return;}
        await route.fulfill({response});return;
      }
      if(request.method()==='GET'&&url.pathname===path&&changed&&!injected){
        injected=true;
        if(name==='readback'){await route.fulfill({status:503,json:{code:'request_failed',message:'受控验收：首次回读失败'}});return;}
        if(name==='status'){const response=await route.fetch();const current=await response.json();await route.fulfill({response,json:{...current,status:'active'}});return;}
      }
      await route.continue();
    });
    await page.goto(base+'/#/c/'+fid+'/growth?tab=self',{waitUntil:'networkidle'});
    await page.getByRole('button',{name:'查看依据',exact:true}).click();
    await page.getByRole('heading',{name:'这条认识从哪里来',exact:true}).waitFor();
    const before=await get(),initialHistory=(await get('/revisions')).items;
    check('initial full v2/v1 snapshots have matching revision',initialHistory.length===2&&initialHistory.every(r=>r.after_value.revision===r.revision));
    if(name==='normal'){
      await enter('确认已核对','取消的复核');await button('取消').click();check('review cancellation has no write',writes().length===0);
      await enter('确认已核对','已阅读当前认识和完整来源');await button('确认已核对').click();await page.getByRole('heading',{name:'这条认识从哪里来',exact:true}).waitFor();
      const reviewed=await get();check('review preserves revision, confidence, evidence and claim',reviewed.revision===before.revision&&reviewed.confidence===before.confidence&&reviewed.claim===before.claim&&JSON.stringify(reviewed.evidence_refs)===JSON.stringify(before.evidence_refs));
      check('review is independent of history',JSON.stringify((await get('/revisions')).items)===JSON.stringify(initialHistory));
      await button('所有者复核记录').click();await detail().getByText('已阅读当前认识和完整来源',{exact:true}).waitFor();await shot('review');
      await enter('不再采用','取消的遗忘');await button('取消').click();check('forget cancellation preserves current',(await get()).revision===2&&writes().length===1);
      await enter('不再采用','停止采用，保留完整来源');await button('确认不再采用').click();await page.getByRole('heading',{name:'这条认识从哪里来',exact:true}).waitFor();
      await history([3,2,1]);await shot('forgotten-full-history');
      const old=detail().locator('.lp-history-item').filter({has:page.getByRole('heading',{name:/^第 1 版/})});
      await old.getByRole('button',{name:'该版认识快照',exact:true}).click();await old.getByRole('button',{name:'完整修订来源',exact:true}).click();
      const historicalText=await old.innerText();check('v1 rendered full snapshot and complete source',historicalText.includes(initialHistory[1].after_value.claim)&&initialHistory[1].after_value.evidence_refs.every(ref=>historicalText.includes(ref))&&historicalText.includes(initialHistory[1].source_window));await shot('v1-source');
      await old.getByRole('button',{name:'恢复此版内容',exact:true}).click();await detail().getByRole('textbox',{name:'认识操作说明',exact:true}).fill('恢复第一版完整内容');await button('确认历史恢复').click();await page.getByRole('heading',{name:'这条认识从哪里来',exact:true}).waitFor();
      const restored=await get();check('restore appends v4 with exact v1 claim and no old review',restored.revision===4&&restored.claim===initialHistory[1].after_value.claim&&restored.status==='active'&&restored.owner_review===null);check('three intentional commands only',writes().length===3);
      // Explicitly inspect the current revision's history after the record changes.
      await history([4,3,2,1]);await shot('restored-full-history');
      await page.reload({waitUntil:'networkidle'});await page.getByRole('button',{name:'查看依据',exact:true}).click();await history([4,3,2,1]);await button('所有者复核记录').click();await detail().getByText('已阅读当前认识和完整来源',{exact:true}).waitFor();await shot('refresh-history-review');
      const writesBeforeFormal=writes().length;
      await page.goto(base+'/#/c/'+fid+'/growth/self/claim/'+fid+'-claim-one',{waitUntil:'networkidle'});
      await page.getByRole('heading',{name:'认识详情与修订',exact:true}).waitFor();
      await page.getByText('逐版修订与恢复',{exact:true}).waitFor();
      for(const n of [4,3,2,1])await page.getByText(new RegExp('^版本 '+n+' · ')).waitFor();
      await page.getByText('已阅读当前认识和完整来源',{exact:true}).waitFor();
      check('formal SelfPage preserves v4/v3/v2/v1 and prior Owner review without writes',writes().length===writesBeforeFormal);
      await shot('formal-self-history',{dialog:false});
    } else if(name==='adopt-refresh'){
      await enter('不再采用','保留未提交说明');
      const external=await context.request.post(base+path+'/forget',{data:{expectedRevision:2,reason:'外部已确认遗忘',idempotencyKey:'external-forget'}});assert.equal(external.status(),200);
      await button('读取最新版本并保留输入').click();await button('已核对，采用最新基线').click();check('latest adoption retains reason',await detail().getByRole('textbox',{name:'认识操作说明',exact:true}).inputValue()==='保留未提交说明');await button('取消').click();
      await detail().getByText('已遗忘',{exact:true}).waitFor();await history([3,2,1]);await shot('adopt-cancel');check('adoption and cancel never mutate',writes().length===0);
      const review=await context.request.post(base+path+'/review',{data:{expectedRevision:3,reason:'外部新复核记录',idempotencyKey:'external-review'}});assert.equal(review.status(),200);
      await button('刷新当前详情').click();await button('所有者复核记录').click();await detail().getByText('外部新复核记录',{exact:true}).waitFor();await shot('refresh-secondary');check('manual refresh stays read-only',writes().length===0);
    } else {
      await enter('不再采用','验证原请求恢复');await button('确认不再采用').click();
      if(name==='unknown'){
        await button('核对或重试原提交').waitFor();check('unknown cannot adopt latest before original admission resolved',await button('读取最新版本并保留输入').isDisabled());await shot('unknown');await button('核对或重试原提交').click();
      } else {await button('只读恢复页面结果').waitFor();await shot('accepted-readback-pending');await button('只读恢复页面结果').click();}
      await page.getByRole('heading',{name:'这条认识从哪里来',exact:true}).waitFor();await detail().getByText('已遗忘',{exact:true}).waitFor();await history([3,2,1]);
      const attempts=writes();check('recovery preserves mutation count',attempts.length===(name==='unknown'?2:1));if(name==='unknown')check('original retry has identical key and body',JSON.stringify(attempts[0].body)===JSON.stringify(attempts[1].body));check('recovery never appends duplicate revision',(await get('/revisions')).items.length===3);await shot('recovered');
    }
    check('no browser exception',record.errors.length===0);record.status='passed';
  } catch(error){record.status='failed';record.error=error.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}
  finally {results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);try{await context.close();}finally{await stop(child);}}
}} finally {await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

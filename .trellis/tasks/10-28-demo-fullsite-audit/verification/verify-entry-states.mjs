// Bounded entry states through the actual production components. Test-only HTTP.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import net from 'node:net';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const port=Number(process.env.ENTRY_STATES_PORT||4457),base=`http://127.0.0.1:${port}`;
assert.ok(Number.isSafeInteger(port)&&port>=1024&&port<=65535);
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/entry-states-'+(process.env.AUDIT_PHASE||'pass109'));
await mkdir(out,{recursive:true});
const probe=net.createServer();await new Promise((ok,no)=>{probe.once('error',no);probe.listen(port,'127.0.0.1',ok);});await new Promise(ok=>probe.close(ok));
const child=spawn(process.execPath,['apps/web/test/fixtures/ui-quality-server.mjs'],{cwd:process.cwd(),env:{...Object.fromEntries(Object.entries(process.env).filter(([k])=>!k.startsWith('FLUCTLIGHT_UI_FIXTURE_'))),FLUCTLIGHT_UI_FIXTURE_PORT:String(port)},stdio:['ignore','pipe','pipe']});
async function stop(){if(child.exitCode!==null||child.signalCode!==null||!child.pid)return;const end=once(child,'exit');child.kill('SIGTERM');await end;}
let browser;
const results=[],cases=['auth-setup','auth-status-recovery','library-draft','library-activated','library-corrupt','library-lifecycle','library-detail-recovery','library-directory-failure'];
try{
  await new Promise((ok,no)=>{let stdout='',stderr='';const timer=setTimeout(()=>no(Error('fixture startup timeout '+stderr)),10000);const fail=e=>{clearTimeout(timer);no(e);};child.once('error',fail);child.once('exit',(code,signal)=>fail(Error('fixture exit '+(signal||code)+': '+stderr)));child.stderr.on('data',v=>{stderr+=v;});child.stdout.on('data',v=>{stdout+=v;if(stdout.split('\n').includes('Isolated UI fixture: '+base)){clearTimeout(timer);ok();}});});
  browser=await chromium.launch({channel:'chrome',headless:true});
  for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of cases){
    if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))continue;
    const context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(10000);
    const record={name,size,width,height,boundary:'production dist / controlled auth and directory HTTP; no real account creation or business mutation',checks:[],requests:[],errors:[],screenshots:[]};
    const gates=[];const gate=()=>{let release;const promise=new Promise(ok=>{release=ok;});gates.push(release);return {promise,release};};
    page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/')||path.startsWith('/auth/'))record.requests.push({path,method:r.method()});});
    const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
    const shot=async label=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(label+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,name+'-'+size+'-'+label+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
    const goto=path=>page.goto(base+'/#'+path,{waitUntil:'domcontentloaded'});
    try{
      if(name.startsWith('auth-')){
        let authenticated=false,setupWrites=0,statusReads=0;
        await page.route(base+'/auth/session',route=>route.fulfill({json:authenticated?{authenticated:true,actorId:'human-fixture-owner'}:{authenticated:false}}));
        const statusGate=gate(),submitGate=gate();
        await page.route(base+'/auth/setup-status',async route=>{statusReads++;if(name==='auth-status-recovery'&&statusReads===1){await statusGate.promise;await route.fulfill({status:503,json:{code:'request_failed'}});}else await route.fulfill({json:{setupAvailable:true}});});
        await page.route(base+'/auth/setup',async route=>{setupWrites++;await submitGate.promise;authenticated=true;await route.fulfill({json:{authenticated:true,actorId:'human-fixture-owner'}});});
        await goto('/auth');
        if(name==='auth-status-recovery'){
          await page.getByRole('button',{name:'正在读取账户状态…',exact:true}).waitFor();
          check('checking disables authentication submit',await page.getByRole('button',{name:'正在读取账户状态…',exact:true}).isDisabled());await shot('checking');statusGate.release();
          await page.getByRole('alert').filter({hasText:'暂时无法读取初始化状态'}).waitFor();await shot('failed');
          await page.getByRole('button',{name:'重新读取',exact:true}).click();await page.getByRole('button',{name:'完成初始化',exact:true}).waitFor();
          check('status retry recovers real setup form without auth POST',statusReads===2&&setupWrites===0);await shot('recovered');
        }else{
          await page.getByRole('button',{name:'完成初始化',exact:true}).waitFor();await shot('normal');
          const password=page.getByLabel('所有者密码',{exact:true}),token=page.getByLabel('初始化令牌',{exact:true}),confirmation=page.getByLabel('再次输入密码',{exact:true});
          await password.fill('fixture-owner-password');await token.fill('short');await confirmation.fill('fixture-owner-password');await page.getByRole('button',{name:'完成初始化',exact:true}).click();
          await page.getByRole('alert').filter({hasText:'请输入有效的初始化令牌'}).waitFor();check('short token prevents POST and receives focus',setupWrites===0&&await token.evaluate(e=>e===document.activeElement));await shot('short-token');
          await token.fill('fixture-setup-token-value');await confirmation.fill('different-password');await page.getByRole('button',{name:'完成初始化',exact:true}).click();
          await page.getByRole('alert').filter({hasText:'两次输入的密码不一致'}).waitFor();check('mismatch prevents POST and focuses confirmation',setupWrites===0&&await confirmation.evaluate(e=>e===document.activeElement));await shot('mismatch');
          await confirmation.fill('fixture-owner-password');const requested=page.waitForRequest(base+'/auth/setup');await page.getByRole('button',{name:'完成初始化',exact:true}).click();await requested;
          await page.getByRole('button',{name:'正在连接…',exact:true}).waitFor();check('busy disables all three credential fields',await password.isDisabled()&&await token.isDisabled()&&await confirmation.isDisabled());check('busy prevents duplicate submit',await page.getByRole('button',{name:'正在连接…',exact:true}).isDisabled()&&setupWrites===1);await shot('busy');submitGate.release();
          await page.getByTestId('replica-library').waitFor();check('acknowledged setup enters actual workspace',setupWrites===1&&await page.locator('.library-card').count()>0);await shot('complete');
        }
      }else{
        if(['library-draft','library-activated','library-corrupt'].includes(name)){
          const draft={version:1,step:name==='library-activated'?3:1,requestId:'fixture-entry-state',name:'仍未发送的名字',mode:'blank',timezone:'Asia/Shanghai',useDefaultWorld:true,activatedId:name==='library-activated'?'fluctlight-fixture-one':null};
          await context.addInitScript(({raw})=>localStorage.setItem('fluctlight-creation:human-fixture-owner:draft',raw),{raw:name==='library-corrupt'?'{unreadable':JSON.stringify(draft)});
        }
        if(name==='library-lifecycle')await page.route(base+'/api/fluctlights',async route=>{const response=await route.fetch(),rows=await response.json(),first=rows[0];await route.fulfill({json:[first,...['paused','retired','failed','preparing'].map(status=>({...first,id:'entry-'+status,status,identity:{...first.identity,name:'验收-'+status}}))]});});
        const detailGate=gate();let detailMode=name==='library-detail-recovery'?'held':'ready',directoryFailure=name==='library-directory-failure';
        if(name==='library-detail-recovery')await page.route(/\/api\/fluctlights\/[^/]+\/detail$/,async route=>{if(detailMode==='held'){await detailGate.promise;await route.fulfill({status:503,json:{code:'request_failed'}});}else await route.continue();});
        if(name==='library-directory-failure')await page.route(base+'/api/fluctlights',async route=>{if(directoryFailure)await route.fulfill({status:503,json:{code:'request_failed'}});else await route.continue();});
        await goto('/library');
        if(name==='library-directory-failure'){
          await page.locator('.approved-bootstrap[role=alert]').waitFor();check('required directory failure never fabricates cards',await page.locator('.library-card').count()===0);await shot('failed');directoryFailure=false;await page.getByRole('button',{name:'重新读取',exact:true}).click();await page.getByTestId('replica-library').waitFor();await page.locator('.library-card').first().waitFor();await shot('recovered');
        }else{
          await page.getByTestId('replica-library').waitFor();await page.locator('.library-card').first().waitFor();
          if(name==='library-detail-recovery'){
            await page.getByText('正在读取角色资料…',{exact:true}).waitFor();check('detail loading retains real directory cards',await page.locator('.library-card').count()===2);await shot('loading');detailMode='failed';detailGate.release();
            await page.getByRole('button',{name:'重新读取角色资料',exact:true}).waitFor();check('detail failure retains existing cards',await page.locator('.library-card').count()===2);await shot('failed');detailMode='ready';await page.getByRole('button',{name:'重新读取角色资料',exact:true}).click();await page.getByRole('button',{name:'重新读取角色资料',exact:true}).waitFor({state:'detached'});check('detail retry clears error and retains card identities',await page.getByTestId('library-card-fluctlight-fixture-one').count()===1);await shot('recovered');
          }else if(name==='library-lifecycle'){
            for(const [status,label] of [['paused','已暂停'],['retired','已退休'],['failed','需要处理'],['preparing','准备中']]){await page.getByTestId('library-card-entry-'+status).locator('.library-status').getByText(label,{exact:true}).waitFor();check(status+' badge follows actual directory status',true);}await shot('lifecycle');
          }else if(name==='library-corrupt'){
            await page.getByRole('alert').filter({hasText:'上次的创建资料需要核对'}).waitFor();check('corrupt draft warning preserves readable directory',await page.locator('.library-card').count()===2);await shot('warning');
          }else{
            await page.getByText(name==='library-activated'?'继续查看准备进度':'还有一段尚未开始的故事。',{exact:true}).waitFor();
            const hero=page.locator('.library-hero').getByRole('link');check('draft hero uses correct creation identity',(await hero.getAttribute('href'))===(name==='library-activated'?'#/library/create?new=1':'#/library/create'));
            check('resume retains original durable slot',await page.locator('.library-draft-notice').getByRole('link',{name:'继续创建',exact:true}).getAttribute('href')==='#/library/create');await shot('draft');
          }
        }
        check('library state verification performs no business write',record.requests.every(r=>r.method==='GET'));
      }
      check('no browser exception',record.errors.length===0);record.status='passed';
    }catch(e){record.status='failed';record.error=e.stack;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}
    finally{gates.forEach(release=>release());await context.close();}
    results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
  }
}finally{await browser?.close();await stop();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

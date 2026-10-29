import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const version=process.env.AUDIT_VERSION||'after',phase=process.env.AUDIT_PHASE||'feedback1';
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/feedback-entry-'+version+'-'+phase);await mkdir(out,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[],fid='fluctlight-fixture-one',owner='human-fixture-owner';
const draftKey=`fluctlight-creation:${owner}:draft`,commandKey=`fluctlight-creation:${owner}:command`;
const cases=['draft','invalid-json','incompatible','incomplete','orphan','mismatch','storage-failure','storage-getter','legacy-orphan','unknown-activation','history-independent','stale-cleared','unconfigured','awaiting-review'];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of cases){
 if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))continue;
 const env={FLUCTLIGHT_UI_FIXTURE_CREATION:'normal',...(version==='before'?{FLUCTLIGHT_UI_FIXTURE_DIST:'/private/tmp/fluctlight-demo-audit-20261028/dist-before-feedback-20261029'}:{})};
 const server=await startOwnedFixture({port:Number(process.env.FEEDBACK_ENTRY_PORT||4473),env});let context,page;
 const record={name,size,width,height,version,boundary:'production build before/after / controlled creation and storage only; no live model or activation',checks:[],requests:[],errors:[],screenshots:[]};
 try{
  context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce',acceptDownloads:true});page=await context.newPage();page.setDefaultTimeout(10000);
  page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
  const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
  const shot=async label=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});const file=resolve(out,name+'-'+size+'-'+label+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);if(version==='after')check(label+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));};
  const base={version:1,step:1,v62Step:'person',requestId:'feedback-original-request',mode:'blank',name:'尚未完成的资料',bio:'保留完整输入',analysis:null,activatedId:null,timezone:'Asia/Shanghai',useDefaultWorld:true,worldChoiceConfirmed:true,includeGoals:true};
  let draft=base,command;
  if(name==='invalid-json'||name==='history-independent')draft='{invalid-original';
  if(name==='incompatible')draft={...base,version:9};
  if(name==='incomplete')draft={...base,analysis:{}};
  if(['orphan','legacy-orphan','mismatch','unknown-activation'].includes(name))command={version:1,idempotencyKey:'feedback-original-key',attempts:1,value:{body:{requestId:base.requestId,initializationMode:'blank_slate',initializationTimezone:'Asia/Shanghai',name:'原提交的角色'},groupId:'',useDefaultWorld:true}};
  if(name==='orphan'||name==='legacy-orphan')draft=null;
  if(name==='mismatch')draft={...base,requestId:'different-unsent-draft'};
  if(name==='unknown-activation')draft={...base,step:3,v62Step:'progress'};
  if(name==='unconfigured'||name==='awaiting-review')draft=null;
  const raw=typeof draft==='string'?draft:draft?JSON.stringify(draft):null,commandRaw=command?JSON.stringify(command):null;
  await context.addInitScript(({draftKey,commandKey,raw,commandRaw,name,owner,fid,base})=>{
   if(raw!==null)localStorage.setItem(draftKey,raw);if(commandRaw!==null)(name==='legacy-orphan'?sessionStorage:localStorage).setItem(commandKey,commandRaw);
   if(name==='history-independent')localStorage.setItem(`fluctlight-creation:${owner}:history:${fid}`,JSON.stringify({...base,step:3,v62Step:'progress',activatedId:fid,activationDate:'2026-10-28',groupConfirmed:true}));
   if(name==='storage-getter'){window.feedbackStorageReadable=false;const original=window.localStorage;Object.defineProperty(window,'localStorage',{configurable:true,get(){if(!window.feedbackStorageReadable)throw new DOMException('Storage getter blocked by controlled test','SecurityError');return original;}});}
   if(name==='storage-failure'){window.feedbackStorageReadable=false;const read=Storage.prototype.getItem;Storage.prototype.getItem=function(key){if(!window.feedbackStorageReadable&&key.startsWith(`fluctlight-creation:${owner}:`))throw new DOMException('Storage blocked by controlled test','SecurityError');return read.call(this,key);};}
  },{draftKey,commandKey,raw,commandRaw,name,owner,fid,base});
  if(name==='unconfigured')await page.route('**/api/configuration-catalog',route=>route.fulfill({json:{endpoints:[],bindings:[],versions:{},effective_values:{}}}));
  if(name==='awaiting-review')await page.route(/\/api\/fluctlights\/creation-character-[^/]+\/(visual-identity|display-avatar)$/,async route=>{const response=await route.fetch(),body=await response.json();if(new URL(route.request().url()).pathname.endsWith('/visual-identity')){body.canonical_asset_id=null;body.character_sheet_asset_id=null;body.session={...body.session,status:'awaiting_review',current_attempt:1};}else body.avatar=null;await route.fulfill({response,json:body});});
  await page.goto(server.base+'/#/library',{waitUntil:'networkidle'});await page.waitForTimeout(80);
  if(name==='awaiting-review'){
   await page.getByRole('link',{name:'开启新的相识',exact:true}).click();await page.getByRole('tab',{name:'日常生活世界',exact:true}).click();await page.getByRole('button',{name:'使用日常生活世界',exact:true}).click();await page.getByTestId('wizard-world-next').click();await page.getByRole('button',{name:/从空白开始/}).click();await page.getByLabel('角色名字',{exact:true}).fill('等待审阅的角色');await page.getByTestId('wizard-analyze').click();await page.getByTestId('wizard-activate').click();await page.getByTestId('prep-step-reference').getByRole('link',{name:'前往审阅候选',exact:true}).waitFor();await page.goto(server.base+'/#/library',{waitUntil:'networkidle'});
  }
  if(version==='before'){
   await shot('initial');record.status='captured';
  }else{
   await page.getByTestId('replica-library').waitFor();const feedback=()=>page.getByTestId('creation-recovery-feedback');
   if(name==='awaiting-review'){
    await feedback().filter({hasText:'候选形象等待你审阅'}).waitFor();check('actual awaiting review is action required',await feedback().getAttribute('data-feedback-kind')==='action-required');check('review links to the actual original preparation',await feedback().getByRole('link',{name:'查看准备进度',exact:true}).count()===1);check('returning does not repeat activation',record.requests.filter(r=>r.method==='POST'&&r.path.endsWith('/activate')).length===1);await shot('review-required');
   }else if(name==='unconfigured'){
    await page.getByRole('link',{name:'开启新的相识',exact:true}).click();await page.getByRole('tab',{name:'日常生活世界',exact:true}).click();await page.getByRole('button',{name:'使用日常生活世界',exact:true}).click();await page.getByTestId('wizard-world-next').click();await page.getByRole('button',{name:/从空白开始/}).waitFor();check('unconfigured entry retains actual blank creation choice',true);await shot('blank-available');
   }else{
    await feedback().waitFor();
    const expected=['draft','stale-cleared','unknown-activation'].includes(name)?'action-required':['storage-failure','storage-getter'].includes(name)?'error':'warning';
    check('feedback severity follows actual recovery state',await feedback().getAttribute('data-feedback-kind')===expected);
    check('nonblocking recovery is not an assertive red error',expected==='error'||await feedback().getAttribute('role')==='note'||await feedback().getAttribute('role')==='status');
    check('library remains available',await page.locator('.library-card').count()===2);await shot('initial');
    if(['invalid-json','incompatible','incomplete','orphan','mismatch','legacy-orphan'].includes(name)){
     check('invalid state has no invented continue action',await feedback().getByRole('link').count()===0);const download=page.waitForEvent('download');await feedback().getByRole('button',{name:'下载原记录',exact:true}).click();await download;check('export is a real download and preserves original bytes',await page.evaluate(key=>localStorage.getItem(key),draftKey)===raw&&await page.evaluate(({key,name})=>(name==='legacy-orphan'?sessionStorage:localStorage).getItem(key),{key:commandKey,name})===commandRaw);await feedback().getByRole('button',{name:'技术详情',exact:true}).click();await feedback().locator('pre').waitFor();await shot('details');await feedback().getByRole('button',{name:'技术详情',exact:true}).click();
    }else if(['storage-failure','storage-getter'].includes(name)){
     await page.evaluate(()=>{window.feedbackStorageReadable=true;});await feedback().getByRole('button',{name:'重新读取本机记录',exact:true}).click();await feedback().getByRole('link',{name:'继续创建',exact:true}).waitFor();check('successful storage reread clears old failure',await feedback().getAttribute('data-feedback-kind')==='action-required');await shot('recovered');
    }else if(name==='stale-cleared'){
     await page.evaluate(({key})=>{localStorage.removeItem(key);window.dispatchEvent(new StorageEvent('storage',{key}));},{key:draftKey});await feedback().waitFor({state:'detached'});check('authoritative missing draft clears obsolete notice',await page.getByRole('link',{name:'开启新的相识',exact:true}).count()===1);await shot('cleared');
    }else if(name==='history-independent'){
     await page.getByTestId('library-card-'+fid).getByRole('link',{name:'查看准备进度',exact:true}).click();await page.locator('.replica-creation.creation-page').waitFor();check('valid archived progress opens independently of corrupt current draft',await page.getByRole('heading',{name:'创建记录暂时无法继续',exact:true}).count()===0);check('current corrupt record is retained',await page.evaluate(key=>localStorage.getItem(key),draftKey)===raw);await shot('history-progress');
    }else if(name==='unknown-activation'){
     await feedback().getByRole('link',{name:'核对创建结果',exact:true}).click();await page.getByRole('button',{name:'查询或重试原激活',exact:true}).waitFor();check('opening unknown does not create another role',!record.requests.some(r=>r.method==='POST'&&r.path.endsWith('/activate')));const originalResponse=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/fluctlight-creations/activate');await page.getByRole('button',{name:'查询或重试原激活',exact:true}).click();await originalResponse;const posts=record.requests.filter(r=>r.method==='POST'&&r.path.endsWith('/activate'));check('explicit recovery replays original activation body once',posts.length===1&&JSON.stringify(posts[0].body)===JSON.stringify(command.value.body));await shot('original-replayed');
    }else{
     check('valid draft offers exactly one continuation CTA',await page.getByRole('link',{name:'继续创建',exact:true}).count()===1);await feedback().getByRole('link',{name:'继续创建',exact:true}).click();await page.locator('.replica-creation.creation-page').waitFor();check('continuation restores actual saved input',await page.getByRole('textbox',{name:'角色名字',exact:true}).inputValue()===base.name);await shot('continued');
    }
   }
   check('no uncaught browser exception',record.errors.length===0);record.status='passed';
  }
 }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
 finally{try{await context?.close();}finally{await server.stop();}}
 results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(version,name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status==='failed'))process.exitCode=1;

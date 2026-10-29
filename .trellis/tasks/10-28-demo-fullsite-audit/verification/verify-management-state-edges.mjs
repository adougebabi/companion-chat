import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/management-state-edges-'+(process.env.AUDIT_PHASE||'pass123'));
await mkdir(out,{recursive:true});const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
async function bounded(promise){let timer;try{return await Promise.race([promise,new Promise((_,no)=>{timer=setTimeout(()=>no(Error('expected controlled request did not start')),12000);})]);}finally{clearTimeout(timer);}}
const cases=['settings-busy','settings-unknown','settings-conflict','settings-rejected','settings-partial','backup-states','import-recovery'];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of cases){
 if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))continue;
 const server=await startOwnedFixture({port:Number(process.env.MANAGEMENT_EDGES_PORT||4471),env:{FLUCTLIGHT_UI_FIXTURE_MANAGEMENT:'normal'}});let context,page,release;
 const record={name,size,width,height,boundary:'production dist / controlled configuration receipts, in-memory jobs and pack HTTP; no real settings, import or maintenance',checks:[],requests:[],errors:[],screenshots:[]};
 try{
  context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(10000);
  page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
  const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);},d=()=>page.getByRole('dialog').last();
  const writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
  const get=async path=>{const response=await context.request.get(server.base+path);assert.equal(response.status(),200,path);return response.json();};
  const go=async path=>{await page.goto(server.base+'/#'+path,{waitUntil:'networkidle'});await page.locator('h1,h2').first().waitFor();};
  const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,name+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
  if(name.startsWith('settings-')){
   const cfg=await get('/api/configuration-catalog');cfg.effective_values={'vision.understanding':{enabled:false},'product.wakeup':{enabled:false}};cfg.versions['vision.understanding']=1;cfg.versions['product.wakeup']=1;
   const receipts=new Map(),commands=[],originals=new Map();let fault=true,arrived;const received=new Promise(ok=>{arrived=ok;}),gate=new Promise(ok=>{release=ok;});
   await page.route('**/api/**',async route=>{
    const req=route.request(),url=new URL(req.url()),path=url.pathname,body=req.postDataJSON();
    if(path==='/api/configuration-catalog'){await route.fulfill({json:cfg});return;}
    if(path.startsWith('/api/configuration-commands/')){
     const key=url.searchParams.get('key')||body?.key;
     if(path.endsWith('/cancel-unaccepted')){assert.ok(!receipts.has(key),'only unaccepted command may cancel');await route.fulfill({json:{status:'cancelled'}});return;}
     const value=receipts.get(key);await route.fulfill(value?{json:value}:{status:404,json:{code:'not_found',message:'受控原命令尚未受理'}});return;
    }
    if(req.method()!=='PUT'||!['/api/settings','/api/providers/endpoints','/api/providers/roles'].includes(path)){await route.continue();return;}
    commands.push({path,body:structuredClone(body)});const key=body.idempotencyKey;
    if(originals.has(key))assert.deepEqual(originals.get(key),{path,body});else originals.set(key,structuredClone({path,body}));
    if(receipts.has(key)){await route.fulfill({json:receipts.get(key)});return;}
    if(path==='/api/settings'&&name==='settings-busy'&&fault){fault=false;arrived();await gate;}
    if(path==='/api/settings'&&fault&&['settings-conflict','settings-rejected','settings-partial'].includes(name)){
     fault=false;if(name==='settings-conflict')cfg.versions['vision.understanding']++;
     await route.fulfill({status:name==='settings-conflict'?409:422,json:{code:name==='settings-conflict'?'revision_conflict':'invalid_request',message:'受控配置原提交被明确拒绝'}});return;
    }
    const keys=path==='/api/settings'?Object.keys(body.values||{}):[path.endsWith('/endpoints')?'endpoint:'+body.endpointId:'binding:'+body.role];
    for(const resource of keys)assert.equal(body.expectedVersions[resource]||0,cfg.versions[resource]||0,'exact configuration CAS '+resource);
    if(path==='/api/settings')Object.assign(cfg.effective_values,structuredClone(body.values));
    else if(path.endsWith('/endpoints')){cfg.endpoints=cfg.endpoints.map(e=>e.id===body.endpointId?{...e,base_url:body.baseUrl,kind:body.kind}:e);cfg.bindings=cfg.bindings.filter(b=>b.endpoint_id!==body.endpointId);}
    else cfg.bindings=cfg.bindings.filter(b=>b.role!==body.role).concat({role:body.role,endpoint_id:body.endpointId,model_id:body.modelId,timeout_seconds:body.timeoutSeconds});
    for(const resource of keys)cfg.versions[resource]=(cfg.versions[resource]||0)+1;
    const receipt={status:'applied',applied_versions:structuredClone(cfg.versions)};receipts.set(key,receipt);
    if(path==='/api/settings'&&name==='settings-unknown'&&fault){fault=false;await route.fulfill({status:503,json:{code:'request_failed',message:'受控配置已提交但回执丢失'}});return;}
    await route.fulfill({json:receipt});
   });
   await go('/settings?section='+encodeURIComponent('看图理解'));const nav=page.getByRole('navigation',{name:'管理中心设置目录'}),vision=()=>page.getByRole('switch',{name:'启用视觉理解',exact:true});await vision().setChecked(true);
   const toVision=()=>nav.getByRole('button',{name:'聊天与视觉',exact:true}).click(),toWake=()=>nav.getByRole('button',{name:'通知与后台',exact:true}).click();
   if(name==='settings-busy'||name==='settings-partial'){await toWake();await page.getByRole('switch',{name:'启用定时唤醒',exact:true}).setChecked(true);}
   if(name==='settings-partial'){await nav.getByRole('button',{name:'模型服务与用途',exact:true}).click();await page.locator('#config-服务地址 input[type=url]').first().fill('https://confirmed-fixture.invalid/v1');}
   await page.getByTestId('config-save').click();
   if(name==='settings-busy'){
    await bounded(received);check('busy blocks repeated save and discard',await page.getByTestId('config-save').isDisabled()&&await page.getByRole('button',{name:'放弃未保存修改',exact:true}).isDisabled());await toVision();await vision().setChecked(false);await shot('busy-new-input');release();release=null;await page.getByTestId('config-save-status').filter({hasText:'有未保存修改'}).waitFor();check('accepted snapshot contains both submitted ordinary settings',commands[0].body.values['vision.understanding'].enabled===true&&commands[0].body.values['product.wakeup'].enabled===true);check('late receipt preserves new input',await vision().isChecked()===false);await page.getByTestId('config-save').click();await page.getByTestId('config-save-status').filter({hasText:'设置已保存'}).waitFor();check('second save submits only unacknowledged edit',commands.length===2&&Object.keys(commands[1].body.values).length===1&&commands[1].body.values['vision.understanding'].enabled===false);await shot('confirmed');
   }else{
    await page.getByRole('button',{name:'核对原提交',exact:true}).waitFor();await shot('failure');
    if(name==='settings-unknown'){
     await toWake();await page.getByRole('switch',{name:'启用定时唤醒',exact:true}).setChecked(true);await page.getByTestId('config-save').click();await page.getByRole('alert').filter({hasText:'原配置提交结果待核对'}).waitFor();check('new draft cannot overwrite unknown original command',commands.length===1);await page.getByRole('button',{name:'核对原提交',exact:true}).click();
    }else{
     check('rejected command did not mutate ordinary setting',cfg.effective_values['vision.understanding'].enabled===false);await page.getByRole('button',{name:'停止未受理提交',exact:true}).click();
    }
    await page.getByRole('button',{name:'读取最新基线（保留输入）',exact:true}).click();await page.getByRole('alert').filter({hasText:'已读取最新基线'}).waitFor();await shot('explicit-rebase');await page.getByTestId('config-save').click();await page.getByTestId('config-save-status').filter({hasText:'设置已保存'}).waitFor();
    const settings=commands.filter(c=>c.path==='/api/settings');check('new confirmed proposal uses an independent identity',settings.length===2&&settings[0].body.idempotencyKey!==settings[1].body.idempotencyKey);
    if(name==='settings-unknown'){check('unknown recovery only reads/recovers original receipt before new proposal',writes().filter(r=>r.path==='/api/configuration-commands/recover').length===1&&cfg.effective_values['product.wakeup'].enabled===true);}
    if(name==='settings-conflict')check('explicit conflict rebase updates actual CAS',settings[1].body.expectedVersions['vision.understanding']>settings[0].body.expectedVersions['vision.understanding']);
    if(name==='settings-partial')check('partial success does not repeat confirmed endpoint or binding',commands.filter(c=>c.path.endsWith('/endpoints')).length===1&&commands.filter(c=>c.path.endsWith('/roles')).length===1&&cfg.endpoints[0].base_url==='https://confirmed-fixture.invalid/v1');
    check('final ordinary settings are actually committed',cfg.effective_values['vision.understanding'].enabled===true);await shot('confirmed');
   }
  }else if(name==='backup-states'){
   const list=await get('/api/ops/backups'),running={...list.items[0],id:'backup-running',status:'running',stage:'copy_database',result:{},steps:[{name:'受控数据库副本阶段',status:'running'}],started_at:list.items[0].created_at,finished_at:null};
   await page.route(/\/api\/ops\/backups(?:[/?].*)?$/,async route=>{const path=new URL(route.request().url()).pathname;if(route.request().method()==='GET'&&path==='/api/ops/backups/backup-running')await route.fulfill({json:running});else if(route.request().method()==='GET'&&path==='/api/ops/backups')await route.fulfill({json:{...list,items:[running,...list.items]}});else await route.continue();});
   await go('/data');await page.locator('a[href="#/data/backup/backup-running"]').click();await page.locator('[data-slot=badge]').filter({hasText:'维护进行中'}).first().waitFor();check('running maintenance offers no cancel retry or restore',await page.getByRole('button',{name:/取消尚未开始|核对后发起新尝试|创建全新隔离恢复任务/}).count()===0);await shot('running');
   await go('/data/backup/backup-queued');await page.getByRole('button',{name:'取消尚未开始的任务',exact:true}).click();await d().getByRole('textbox',{name:'操作理由',exact:true}).fill('只取消受控排队维护任务');await d().getByRole('button',{name:'确认提交',exact:true}).click();await page.locator('[data-slot=badge]').filter({hasText:'已取消'}).first().waitFor();check('cancelled readback has no new maintenance action',await page.getByRole('button',{name:/取消尚未开始|核对后发起新尝试|创建全新隔离恢复任务/}).count()===0);const cancel=writes().filter(r=>r.path==='/api/ops/backups/backup-queued/cancel');check('queued cancel uses original revision once',cancel.length===1&&cancel[0].body.expectedRevision===2&&(await get('/api/ops/backups/backup-queued')).status==='cancelled');await shot('cancelled');
  }else{
   const gate=new Promise(ok=>{release=ok;});let firstValidation=true,arrived,lost=false;const received=new Promise(ok=>{arrived=ok;});
   await page.route('**/api/studio/packs/validate',async route=>{if(firstValidation){firstValidation=false;arrived();await gate;}await route.continue();});
   await page.route('**/api/studio/packs',async route=>{if(route.request().method()==='POST'&&!lost){lost=true;const response=await route.fetch();assert.equal(response.status(),200);await route.fulfill({status:503,json:{code:'request_failed',message:'受控导入回执丢失'}});}else await route.continue();});
   await go('/data');const input=page.getByRole('textbox',{name:'正式定义包 JSON',exact:true}),pack={schema:'fluctlight.resource-pack.v1',created_at:'2026-10-27T00:00:00Z',checksum:'fixture-checksum',resources:[{key:'resource-fixture',kind:'item',name:'窗边书签',body:{description:'受控原定义'}}],groups:[],preferences:{}};await input.fill(JSON.stringify(pack));await page.getByRole('button',{name:'校验',exact:true}).click();await bounded(received);check('validation busy blocks duplicate request',await page.getByRole('button',{name:'校验',exact:true}).isDisabled());pack.resources[0].body.description='校验期间的新草稿';await input.fill(JSON.stringify(pack));release();release=null;await page.getByText('校验期间草稿已变化，请对当前内容重新校验。',{exact:true}).waitFor();check('changed validation cannot import old snapshot',await page.getByRole('button',{name:'导入为独立草稿',exact:true}).count()===0);await shot('changed-during-validation');await page.getByRole('button',{name:'校验',exact:true}).click();await page.getByRole('button',{name:'导入为独立草稿',exact:true}).click();await d().getByRole('textbox',{name:'导入理由',exact:true}).fill('仅导入为独立测试草稿');await d().getByRole('button',{name:'确认提交',exact:true}).click();await d().getByRole('button',{name:'核对或重试原提交',exact:true}).waitFor();await shot('unknown');await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});await page.reload({waitUntil:'networkidle'});await page.getByRole('button',{name:'核对或重试原导入',exact:true}).click();await d().getByRole('button',{name:'核对或重试原提交',exact:true}).click();await page.waitForURL('**/#/data/pack/pack-fixture');await page.getByRole('heading',{name:/已导入的定义草稿/}).waitFor();await page.getByText('窗边书签',{exact:true}).waitFor();const imported=writes().filter(r=>r.path==='/api/studio/packs');assert.equal(imported.length,2);assert.deepEqual(imported[0].body,imported[1].body);assert.deepEqual(imported[0].body.pack,pack);record.checks.push('unknown import remount replays exact original pack/digest/reason/key');check('import details link to independent unpublished draft',await page.getByRole('link',{name:/窗边书签/}).getAttribute('href')==='#/studio/edit/resource-fixture'&&await page.locator('[data-slot=badge]').filter({hasText:'尚未发布'}).count()===1);check('import never publishes or applies',!writes().some(r=>r.path.endsWith('/publish')||r.path.endsWith('/apply')));await shot('independent-draft');await page.getByRole('button',{name:'重新读取',exact:true}).click();check('pack detail refresh does not import again',writes().filter(r=>r.path==='/api/studio/packs').length===2);
  }
  check('no browser exception',record.errors.length===0);record.status='passed';
 }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
 finally{release?.();try{await context?.close();}finally{await server.stop();}}
 results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

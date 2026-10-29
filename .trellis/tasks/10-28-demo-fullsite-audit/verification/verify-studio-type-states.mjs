import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/studio-type-states-'+(process.env.AUDIT_PHASE||'pass112'));
await mkdir(out,{recursive:true});const results=[],browser=await chromium.launch({channel:'chrome',headless:true});
const labels={item:['物品描述','description'],outfit:['搭配说明','description'],place:['氛围','atmosphere'],npc:['人物背景','background'],event_template:['事件描述','description'],persona_fragment:['昵称',null],visual_style:['一致性约束','constraints']};
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const kind of Object.keys(labels)){
  if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(kind))continue;
  const server=await startOwnedFixture({port:Number(process.env.STUDIO_TYPES_PORT||4461),env:{FLUCTLIGHT_UI_FIXTURE_STUDIO:'normal'}});let context,page;
  const record={name:kind,size,width,height,boundary:'production dist / controlled saved resource HTTP; no publication, runtime or model success claim',checks:[],requests:[],errors:[],screenshots:[]};
  try{
    context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(10000);
    page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
    const api='/api/studio/resources/studio-'+kind,check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
    const get=async()=>{const r=await context.request.get(server.base+api);assert.equal(r.status(),200);return r.json();};
    const writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
    const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,kind+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
    const before=await get();await page.goto(server.base+'/#/studio/edit/studio-'+kind,{waitUntil:'networkidle'});await page.getByRole('textbox',{name:'资源名称',exact:true}).waitFor();await shot('normal');
    await page.getByRole('textbox',{name:'保存原因',exact:true}).fill('核对类型字段与完整JSON保留');
    await page.getByRole('button',{name:'完整 JSON 定义',exact:true}).click();
    const json=()=>page.getByRole('textbox',{name:'完整 JSON 定义',exact:true});await json().fill('{unreadable');
    await page.getByRole('button',{name:'核对并采用 JSON 输入',exact:true}).click();await page.getByRole('alert').first().waitFor();
    check('invalid JSON retains exact unsent text',await json().inputValue()==='{unreadable');
    await page.getByRole('button',{name:'保存草稿',exact:true}).first().click();check('invalid JSON cannot write or change saved record',writes().length===0&&JSON.stringify((await get()).body)===JSON.stringify(before.body));await shot('invalid-json');
    const extended=structuredClone(before.body);extended.audit_extension={nested:[false,0,{text:'原扩展'}]};
    if(kind==='persona_fragment')extended.changes.identity.nickname='类型编辑前的称呼';
    await json().fill(JSON.stringify(extended,null,2));
    const [label,key]=labels[kind];
    if(kind!=='persona_fragment')check('unadopted JSON fences structured edits',await page.getByRole('textbox',{name:label,exact:true}).isDisabled());
    await page.getByRole('button',{name:'核对并采用 JSON 输入',exact:true}).click();
    check('explicit adoption clears corrected local JSON failure',await page.getByRole('alert').count()===0);
    if(kind==='persona_fragment')await page.getByRole('button',{name:'身份资料',exact:true}).click();
    const field=()=>page.getByRole('textbox',{name:label,exact:true}),next='类型字段修改 '+kind+' '+size;
    check('adopted definition exposes editable typed field',await field().isEnabled());await field().fill(next);
    if(kind==='outfit'){
      await page.getByRole('combobox',{name:'服饰定义',exact:true}).click();await page.getByRole('option').filter({hasText:'亚麻衬衫'}).click();await page.getByRole('button',{name:'加入搭配成员',exact:true}).click();
    }
    if(kind==='event_template'){
      await page.getByRole('combobox',{name:'事件强度',exact:true}).click();await page.getByRole('option',{name:'中',exact:true}).click();
      await page.getByRole('button',{name:'可能结果与范围',exact:true}).click();await page.getByRole('button',{name:'添加效果',exact:true}).click();
    }
    if(kind==='item'){
      await page.getByRole('combobox',{name:'物品类型',exact:true}).click();await page.getByRole('option',{name:'普通物品',exact:true}).click();
      check('ordinary item removes wearing-slot control',await page.getByRole('combobox',{name:'穿着部位',exact:true}).count()===0);
    }
    // Keep a field-focused editing screenshot, separate from JSON validation.
    await field().scrollIntoViewIfNeeded();await shot('editing');
    const savedResponse=page.waitForResponse(r=>new URL(r.url()).pathname===api&&r.request().method()==='PUT');await page.getByRole('button',{name:'保存草稿',exact:true}).first().click();assert.equal((await savedResponse).status(),200);
    await page.getByText('定义草稿已保存并读回。',{exact:true}).waitFor();
    const after=await get(),actual=key?after.body[key]:after.body.changes.identity.nickname;
    check('typed field saved to exact owning definition',actual===next);
    assert.deepEqual(after.body.audit_extension,extended.audit_extension);record.checks.push('unknown nested extension, false and zero preserved');
    check('save remains separate from publication',after.published_version===before.published_version&&after.draft_version!==before.draft_version);
    if(kind==='outfit')check('member saves exact immutable resource version',after.body.members.length===1&&after.body.members[0].resource_id==='studio-item'&&after.body.members[0].resource_version==='draft:1');
    if(kind==='event_template')check('typed event fields preserve original condition and new effect',after.body.intensity==='medium'&&after.body.effects.length===1&&JSON.stringify(after.body.condition)===JSON.stringify(before.body.condition));
    if(kind==='item')check('ordinary item saves no wearing slot',after.body.item_kind==='object'&&after.body.slot==='');
    await page.reload({waitUntil:'networkidle'});if(kind==='persona_fragment')await page.getByRole('button',{name:'身份资料',exact:true}).click();check('refresh reopens actual confirmed typed value',await field().inputValue()===next);await field().scrollIntoViewIfNeeded();await shot('saved-reopened');
    check('one definition PUT and no automatic publish or apply',writes().length===1&&writes()[0].method==='PUT'&&writes()[0].path===api);check('no browser exception',record.errors.length===0);record.status='passed';
  }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,kind+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
  finally{try{await context?.close();}finally{await server.stop();}}
  results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(kind,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

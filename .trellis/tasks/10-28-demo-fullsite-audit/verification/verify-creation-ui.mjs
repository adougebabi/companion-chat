import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {readFile,mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const {data}=JSON.parse(await readFile('apps/web/test/fixtures/demo-parity-data.json','utf8'));
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/creation');await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];const selected=process.env.AUDIT_CASES?.split(',');
async function run(name,fn,size={width:1348,height:925}){
 if(selected&&!selected.includes(name))return;
 const context=await browser.newContext({viewport:size,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(8000);
 const record={name,viewport:size,kind:'production wizard / controlled same-content analysis, no activation or backend preparation certified',checks:[],errors:[],writes:[]};page.on('pageerror',e=>record.errors.push(e.message));page.on('request',request=>{if(request.method()==='POST'&&request.url().includes('/api/'))record.writes.push({url:request.url(),body:request.postDataJSON()});});
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
 const shot=async state=>{const path=resolve(root,name+'-'+state+'.png');await page.screenshot({path});(record.screenshots||=[]).push(path);};
 try{await fn({page,check,shot,record});check('no activation was submitted',!record.writes.some(r=>r.url.includes('/activate')));check('no browser exception',record.errors.length===0);check('no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));record.status='passed';}catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}finally{results.push(record);console.log(name,record.status,record.error||record.checks.length);await writeFile(resolve(root,'creation-ui'+(selected?'-'+selected.join('-'):'')+'.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}
async function start(page){await page.goto('http://127.0.0.1:4381/#/library/create',{waitUntil:'networkidle'});await page.getByRole('tab',{name:'日常生活世界',exact:true}).click();await page.getByRole('button',{name:'使用日常生活世界',exact:true}).click();await page.getByTestId('wizard-world-next').click();await page.getByLabel('角色名字',{exact:true}).waitFor();}
try{
 for(const mode of ['blank','description','json','template'])for(const size of [{name:'desktop',width:1348,height:925},{name:'mobile',width:390,height:844}])await run('source-'+mode+'-'+size.name,async({page,check,shot,record})=>{
  await start(page);await page.getByRole('button',{name:({blank:/从空白开始/,description:/一段描述/,json:/导入 JSON/,template:/角色模板/})[mode]}).click();await page.getByLabel('角色名字',{exact:true}).fill('林晚');
  if(mode==='description')await page.getByLabel('角色描述',{exact:true}).fill(data.creationDescription);
  if(mode==='json')await page.getByLabel('创建 JSON',{exact:true}).fill(JSON.stringify(data.creationFoundation,null,2));
  if(mode==='template'){await page.locator('.creation-template-list button').first().click();}
  await shot('input');await page.getByTestId('wizard-analyze').click();await page.getByTestId('wizard-activate').waitFor();await shot('preview');
  if(mode==='blank')check('blank source skips analysis',record.writes.length===0);
  else{
   check('analysis used the selected source',record.writes.some(r=>r.url.endsWith('/analysis')));
   for(const label of ['性格与表达','生活设定','人格与关系','完整设定']){await page.getByRole('tab',{name:label,exact:true}).click();await shot(label);}
   const json=page.getByRole('textbox',{name:'完整设定 JSON',exact:true});const definition=JSON.parse(await json.inputValue());check('full nested identity retained',definition.core_persona.identity.name==='林晚');check('full life settings retained',definition.core_persona.life_profile.appearance.description===data.creationFoundation.core_persona.life_profile.appearance.description);
   const before=record.writes.length;await page.reload({waitUntil:'networkidle'});await page.getByTestId('wizard-activate').waitFor();check('reload restores preview without another analysis',record.writes.length===before);await shot('restored');
  }
  await page.getByRole('button',{name:'上一步',exact:true}).click();await page.getByLabel('角色名字',{exact:true}).waitFor();check('back preserves entered name',await page.getByLabel('角色名字',{exact:true}).inputValue()==='林晚');
 },size);
 await run('preview-trait-edit-preservation',async({page,check,shot})=>{
  await start(page);await page.getByRole('button',{name:/一段描述/}).click();await page.getByLabel('角色名字',{exact:true}).fill('林晚');await page.getByLabel('角色描述',{exact:true}).fill(data.creationDescription);await page.getByTestId('wizard-analyze').click();await page.getByTestId('wizard-activate').waitFor();
  await page.getByRole('tab',{name:'性格与表达',exact:true}).click();const slider=page.getByRole('slider',{name:'开放性',exact:true});check('existing personality score is editable',await slider.isEnabled());await slider.focus();await slider.press('ArrowRight');await shot('edited-trait');
  await page.getByRole('tab',{name:'完整设定',exact:true}).click();let definition=JSON.parse(await page.getByRole('textbox',{name:'完整设定 JSON',exact:true}).inputValue());check('keyboard edit reaches full preview JSON',definition.core_persona.personality.openness===.73);check('trait edit retains nested life fields',JSON.stringify(definition.core_persona.life_profile)===JSON.stringify(data.creationFoundation.core_persona.life_profile));check('trait edit retains relationships',JSON.stringify(definition.initial_relationships)===JSON.stringify(data.creationFoundation.initial_relationships));check('trait edit retains unknown extension',JSON.stringify(definition.extensions)===JSON.stringify(data.creationFoundation.extensions));
  await page.reload({waitUntil:'networkidle'});await page.getByTestId('wizard-activate').waitFor();await page.getByRole('tab',{name:'完整设定',exact:true}).click();definition=JSON.parse(await page.getByRole('textbox',{name:'完整设定 JSON',exact:true}).inputValue());check('edited personality score survives reload',definition.core_persona.personality.openness===.73);
 },{width:390,height:844});
 await run('analysis-failure-recovery',async({page,check,shot,record})=>{
  let failed=true;await page.route('**/api/fluctlight-creations/analysis',route=>failed?route.fulfill({status:503,json:{code:'analysis_unavailable',message:'专项分析读取失败'}}):route.continue());
  await start(page);await page.getByRole('button',{name:/一段描述/}).click();await page.getByLabel('角色名字',{exact:true}).fill('林晚');await page.getByLabel('角色描述',{exact:true}).fill(data.creationDescription);await page.getByTestId('wizard-analyze').click();await page.getByRole('alert').waitFor();await shot('failure');check('description survives failed analysis',await page.getByLabel('角色描述',{exact:true}).inputValue()===data.creationDescription);failed=false;await page.getByTestId('wizard-analyze').click();await page.getByTestId('wizard-activate').waitFor();check('analysis retry returns to review',true);await shot('recovered');
 });
 await run('analysis-loading-cancel',async({page,check,shot})=>{
  let release;const pending=new Promise(r=>release=r);await page.route('**/api/fluctlight-creations/analysis',async route=>{await pending;await route.continue().catch(()=>{});});
  try{await start(page);await page.getByRole('button',{name:/一段描述/}).click();await page.getByLabel('角色名字',{exact:true}).fill('林晚');await page.getByLabel('角色描述',{exact:true}).fill(data.creationDescription);await page.getByTestId('wizard-analyze').click();await page.getByText('正在整理这次的角色设定…',{exact:true}).waitFor();await shot('loading');await page.getByRole('button',{name:'停止本次等待',exact:true}).click();check('cancel returns input control',await page.getByLabel('角色描述',{exact:true}).isEnabled());check('cancel preserves original description',await page.getByLabel('角色描述',{exact:true}).inputValue()===data.creationDescription);await shot('cancelled');}finally{release();}
 });
}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

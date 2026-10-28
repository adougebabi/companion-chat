import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {readFile,mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const {data}=JSON.parse(await readFile('apps/web/test/fixtures/demo-parity-data.json','utf8'));
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/interaction');await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];const selected=process.env.AUDIT_CASES?.split(',');
async function run(name,fn,size={width:1348,height:925}){
 if(selected&&!selected.includes(name))return;
 const context=await browser.newContext({viewport:size,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(8000);
 const record={name,viewport:size,kind:'production dist / controlled HTTP',checks:[],errors:[]};page.on('pageerror',e=>record.errors.push(e.message));
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
 const shot=async state=>{const path=resolve(root,name+'-'+state+'.png');await page.screenshot({path});(record.screenshots||=[]).push(path);};
 try{await fn({page,check,shot});check('no browser exception',record.errors.length===0);check('no document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));record.status='passed';}catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}finally{results.push(record);console.log(name,record.status,record.error||record.checks.length);await writeFile(resolve(root,'settings-studio-ui'+(selected?'-'+selected.join('-'):'')+'.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}
const go=async(page,path)=>{await page.goto('http://127.0.0.1:4381/#'+path,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
try{
 for(const size of [{name:'desktop',width:1348,height:925},{name:'mobile',width:390,height:844}])await run('settings-sections-'+size.name,async({page,check,shot})=>{
  await go(page,'/settings');await page.locator('#config-服务地址 input[type=url]').first().waitFor();const url=page.locator('#config-服务地址 input[type=url]').first(),original=await url.inputValue();await url.fill('https://unsaved.example.test/v1');await shot('models-dirty');
  for(const [category,expected] of [['聊天与视觉',['聊天回复','看图理解']],['图片生成',['图片（ComfyUI）']],['自主性与安静时段',['角色自主与主动联系','安静时段']],['通知与后台',['作息与后台节奏','资源与预算','通知']],['高级设置',['高级']]]){
   await page.getByRole('navigation',{name:'管理中心设置目录'}).getByRole('button',{name:category,exact:true}).click();
   await page.waitForFunction(expected=>{const visible=[...document.querySelectorAll('.settings-group')].filter(e=>getComputedStyle(e).display!=='none');return visible.length===expected.length&&visible.every(e=>expected.includes(e.getAttribute('data-section')));},expected);
   check(category+' exposes only its own sections',true);await shot(category);
  }
  await page.getByRole('navigation',{name:'管理中心设置目录'}).getByRole('button',{name:'模型服务与用途',exact:true}).click();check('connection draft survives section changes',await url.inputValue()==='https://unsaved.example.test/v1');
  await page.getByRole('tab',{name:/用途分配/}).click();await page.locator('#config-模型用途').waitFor({state:'visible'});await shot('roles');check('four real purpose slots render',(await page.locator('#config-模型用途').innerText()).includes('记忆检索向量'));
  await page.getByRole('button',{name:'放弃未保存修改',exact:true}).click();await page.locator('#config-服务地址').waitFor({state:'visible'});check('discard returns to confirmed connection',await url.inputValue()===original);
 },size);
 for(const kind of ['world','character_template','item','outfit','place','npc','event_template','persona_fragment','visual_style'])await run('studio-editor-'+kind,async({page,check,shot})=>{
  const resource=data.resources.find(r=>r.kind===kind),route=resource?'/studio/edit/'+resource.id:'/studio/edit/new?kind='+kind;
  await go(page,route);await page.getByRole('button',{name:'保存草稿',exact:true}).first().waitFor();await shot(resource?'saved-definition':'new-definition');
  check('real editor has structured fields',await page.locator('input,textarea').count()>0);check('complete JSON definition remains reachable',await page.getByText('完整 JSON 定义',{exact:true}).count()>0);
  if(resource)check('resource title is preserved',(await page.locator('h1').innerText()).includes(resource.name));
 });
 for(const width of [700,701,900,1600])await run('shell-boundary-'+width,async({page,check,shot})=>{
  await go(page,'/library');const mobile=width<=700;check('correct navigation mode at breakpoint',await page.getByRole('button',{name:mobile?'打开导航':'收起导航',exact:true}).count()===1);await shot('normal');
 },{width,height:925});
 await run('auth-validation-failure',async({page,check,shot})=>{
  await page.route('**/auth/session',route=>route.fulfill({json:{authenticated:false}}));await page.route('**/auth/setup-status',route=>route.fulfill({json:{setupAvailable:false}}));let writes=0;
  await page.route('**/auth/login',route=>{writes++;return route.fulfill({status:401,json:{error:'fixed-scene bad password'}});});
  await page.goto('http://127.0.0.1:4381/#/auth',{waitUntil:'networkidle'});await page.getByRole('heading',{name:'欢迎回来',exact:true}).waitFor();await page.getByLabel('所有者密码',{exact:true}).waitFor();await shot('normal');const password=page.getByLabel('所有者密码',{exact:true});await password.fill('short');await page.getByRole('button',{name:/进入工作区|登录/}).last().click();
  check('short password does not reach authentication',writes===0);check('validation returns focus to password',await password.evaluate(e=>e===document.activeElement));await password.fill('fixed-scene-invalid-password');await page.getByRole('button',{name:/进入工作区|登录/}).last().click();await page.getByRole('alert').waitFor();await shot('rejected');check('authentication failure preserves input',await password.inputValue()==='fixed-scene-invalid-password');check('failure came from actual request',writes===1);
 },{width:390,height:844});
}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;

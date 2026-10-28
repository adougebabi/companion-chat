// Latest production UI + the existing isolated Core/PG owner. Read interaction
// only; credentials stay in privateRoot and are never written into evidence.
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const privateRoot='/private/tmp/fluctlight-demo-audit-20261028',base=new URL('http://127.0.0.1:18480');
const require=createRequire(privateRoot+'/browser-tools/package.json'),{chromium}=require('playwright-core');
const {importCookies}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/runtime.mjs'));
const cookies=importCookies(await readFile(privateRoot+'/cookies.txt','utf8'),base);
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/real-browser');await mkdir(out,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),report={origin:base.origin,layer:'latest production dist / actual isolated Core / existing PostgreSQL records',capturedAt:new Date().toISOString(),checks:[],reads:[],pages:[],errors:[]};
try{
 const context=await browser.newContext({viewport:{width:1348,height:925},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
 await context.addCookies([...cookies].map(([name,value])=>({name,value,url:base.origin,httpOnly:name!=='fluctlight_csrf'})));
 const session=await context.request.get(base.origin+'/auth/session');assert.equal(session.status(),200);assert.equal((await session.json()).authenticated,true);report.checks.push('existing isolated Owner session authenticated');
 const directoryResponse=await context.request.get(base.origin+'/api/fluctlights');assert.equal(directoryResponse.status(),200);const body=await directoryResponse.json(),characters=Array.isArray(body)?body:body.items;assert.ok(characters.length>0);const role=characters.find(c=>c.status==='active')||characters[0];assert.ok(role.id);report.character={id:role.id,name:role.identity?.name||role.name};
 const page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));page.on('response',response=>{const url=new URL(response.url());if(url.origin===base.origin&&url.pathname.startsWith('/api/'))report.reads.push({method:response.request().method(),path:url.pathname,status:response.status()});});
 for(const [name,path,ready] of [['library','/library','.library-card'],['chat','/c/'+role.id+'/chat','.companion-chat-heading'],['now','/c/'+role.id+'/life/now','.lp-hero'],['studio','/studio','.r-resource-card'],['inbox','/inbox','.replica-inbox-toolbar']]){
  await page.goto(base.origin+'/#'+path,{waitUntil:'networkidle'});await page.locator(ready).first().waitFor();const shot=resolve(out,name+'.png');await page.screenshot({path:shot});report.uiEntry=await page.locator('script[type=module][src]').first().getAttribute('src');report.pages.push({name,path,heading:await page.locator('h1').first().innerText(),screenshot:shot});
  if(name==='library')assert.ok((await page.locator('.library-card').allTextContents()).some(text=>text.includes(report.character.name)),'library must render the actual API character');
  if(name==='chat')assert.ok((await page.locator('.companion-chat-heading').innerText()).includes(report.character.name),'chat must render the selected actual role');
 }
 assert.equal(report.errors.length,0);report.checks.push('five latest React pages rendered actual API data without browser exception','selected real directory role matches library and chat','no chat send, model generation or new activation submitted');report.status='passed';await context.close();
}catch(e){report.status='failed';report.error=e.message;process.exitCode=1;}finally{await browser.close();await writeFile(resolve(out,'report.json'),JSON.stringify(report,null,2)+'\n');}
console.log(JSON.stringify({status:report.status,pages:report.pages.length,checks:report.checks,error:report.error},null,2));

import {createRequire} from 'node:module';
import {mkdir,readFile,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core'),{PNG}=require('pngjs');
const pixelmatch=(await import(require.resolve('pixelmatch'))).default;
const {data}=JSON.parse(await readFile('apps/web/test/fixtures/demo-parity-data.json','utf8'));
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/visual/creation-preview'+(process.env.AUDIT_CAPTURE_PHASE?'-'+process.env.AUDIT_CAPTURE_PHASE:''));await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const size of [{name:'desktop',width:1348,height:925},{name:'mobile',width:390,height:844}]){
 const context=await browser.newContext({viewport:size,deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
 for(const kind of ['reference','actual']){
  const page=await context.newPage();page.setDefaultTimeout(10000);const record={kind,size,shots:[],errors:[]};page.on('pageerror',e=>record.errors.push(e.message));
  try{
   await page.goto((kind==='reference'?'http://127.0.0.1:4179/fluctlight-offline.html':'http://127.0.0.1:4381/')+'#/library/create',{waitUntil:'networkidle'});
   await page.getByRole('tab',{name:'日常生活世界',exact:true}).click();await page.getByRole('button',{name:'使用日常生活世界',exact:true}).click();await page.getByRole('button',{name:'继续，认识角色',exact:true}).click();
   await page.getByRole('button',{name:/一段描述/}).click();await page.getByLabel(/角色名字/).fill('林晚');await page.getByRole('textbox',{name:'角色描述',exact:true}).fill(data.creationDescription);await page.getByRole('button',{name:'整理并预览',exact:true}).click();await page.getByRole('button',{name:'确认，准备相遇',exact:true}).waitFor();if(kind==='reference'){const toast=page.locator('[data-sonner-toast]').first();if(await toast.count())await toast.locator('button').first().click();await toast.waitFor({state:'hidden'});}
   for(const [id,label] of [['identity','身份与背景'],['personality','性格与表达'],['life','生活设定'],['relationships','人格与关系'],['json','完整设定']]){
    await page.getByRole('tab',{name:label,exact:true}).click();await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));document.querySelector('.workspace-page,.main-page')?.scrollTo({top:0});});
    const filename=kind+'-'+id+'-'+size.name+'.png';await page.screenshot({path:resolve(root,filename),animations:'disabled'});record.shots.push(filename);
   }
   record.status='captured';
  }catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText();await page.screenshot({path:resolve(root,kind+'-failure-'+size.name+'.png')});}
  results.push(record);await page.close();
 }
 await context.close();
 for(const id of ['identity','personality','life','relationships','json']){
  try{const a=PNG.sync.read(await readFile(resolve(root,'reference-'+id+'-'+size.name+'.png'))),b=PNG.sync.read(await readFile(resolve(root,'actual-'+id+'-'+size.name+'.png'))),pair=new PNG({width:a.width*2,height:a.height}),diff=new PNG({width:a.width,height:a.height});for(let y=0;y<a.height;y++){a.data.copy(pair.data,y*pair.width*4,y*a.width*4,(y+1)*a.width*4);b.data.copy(pair.data,y*pair.width*4+a.width*4,y*b.width*4,(y+1)*b.width*4);}pixelmatch(a.data,b.data,diff.data,a.width,a.height);await writeFile(resolve(root,'comparison-'+id+'-'+size.name+'.png'),PNG.sync.write(pair));await writeFile(resolve(root,'difference-'+id+'-'+size.name+'.png'),PNG.sync.write(diff));}catch{}
 }
 console.log(size.name,results.slice(-2).map(r=>[r.kind,r.status,r.error||'']));await writeFile(resolve(root,'capture.json'),JSON.stringify(results,null,2)+'\n');
}}finally{await browser.close();}

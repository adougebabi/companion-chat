import {createRequire} from 'node:module';
import {writeFile,mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {primeReferenceWearing} from './reference-scene.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core');
const phase=process.env.AUDIT_CAPTURE_PHASE||'before',results=[];
const routes={profile:'/c/linwan/profile',chat:'/c/linwan/chat',now:'/c/linwan/life/now',wardrobe:'/c/linwan/life/wardrobe'};
const selectors={profile:['.companion-profile-hero','.companion-profile-intro','.companion-profile-intro > p','.companion-tag-row','.companion-tag-row > *','.companion-profile-quote','.companion-profile-quote > p','.companion-profile-quote > :last-child','[data-slot="progress-indicator"]'],chat:['.companion-message-body','.companion-bubble','.companion-message-image','.companion-message-image > *','.companion-message-image-preview','.companion-bubble p'],now:['.lp-timeline-item','.lp-timeline-card','.lp-timeline-card h3','.lp-timeline-card p'],wardrobe:['.r-wearing-strip','.r-wearing-strip p','.r-tabs','.r-toolbar','.r-garment-grid']};
const browser=await chromium.launch({channel:'chrome',headless:true});
try {
 for(const width of [1348,390]){
  const context=await browser.newContext({viewport:{width,height:width===390?844:925},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
  const reference=await context.newPage(),actual=await context.newPage();await primeReferenceWearing(reference);
  for(const [name,route] of Object.entries(routes)){
   const record={name,width};
   for(const [kind,page,base] of [['reference',reference,'http://127.0.0.1:4179/fluctlight-offline.html#'],['actual',actual,'http://127.0.0.1:4381/#']]){
    await page.goto(base+route,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();
    await page.mouse.move(width-1,width===390?843:924);
    await page.evaluate(async()=>{window.scrollTo({top:0,left:0,behavior:'instant'});document.querySelector('.workspace-page')?.scrollTo({top:0,left:0,behavior:'instant'});await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});
    if(name==='chat')await page.locator('.companion-message-list').evaluate(e=>e.scrollTo({top:e.scrollHeight,behavior:'instant'}));
    record[kind]=await page.evaluate(selectors=>Object.fromEntries(selectors.map(selector=>[selector,Array.from(document.querySelectorAll(selector)).map(e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return {text:e.textContent?.slice(0,120),class:e.className,x:r.x,y:r.y,width:r.width,height:r.height,fontSize:s.fontSize,fontWeight:s.fontWeight,lineHeight:s.lineHeight,color:s.color,background:s.backgroundColor,padding:s.padding,margin:s.margin,gap:s.gap,display:s.display,flex:s.flex,flexShrink:s.flexShrink,maxWidth:s.maxWidth,whiteSpace:s.whiteSpace};})])),selectors[name]);
   }
   results.push(record);
  }
  await context.close();
 }
}finally{await browser.close();}
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/normal-geometry');await mkdir(out,{recursive:true});await writeFile(resolve(out,phase+'.json'),JSON.stringify(results,null,2)+'\n');
console.log('Recorded',results.length,'paired geometry samples:',phase);

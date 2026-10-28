import {createRequire} from 'node:module';import {writeFile} from 'node:fs/promises';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const b=await chromium.launch({channel:'chrome',headless:true}),result={};
try{const c=await b.newContext({viewport:{width:1348,height:925},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai'});
 for(const [kind,url] of Object.entries({reference:'http://127.0.0.1:4179/fluctlight-offline.html#/c/linwan/life/now',actual:'http://127.0.0.1:4381/#/c/linwan/life/now'})){
 const p=await c.newPage();await p.goto(url,{waitUntil:'networkidle'});result[kind]=await p.evaluate(()=>{
 const selectors=['html','body','.workspace-switch','.replica-workspace-switch','.nav-section-label','.replica-nav-label','.nav-item','.replica-nav-item','.page-heading','.page-heading h1','.replica-page-heading','.replica-page-heading h1','.eyebrow','.replica-eyebrow','.lp-grid','.lp-hero','.lp-hero h2','.lp-hero-art','.lp-day-stamp','.app-shell','.main','.main-content','.workspace-page','.replica-page','.character-avatar','.avatar'];
 const properties=['font-family','font-size','font-weight','line-height','color','height','min-height','width','grid-template-columns','padding','margin','gap','display','border','box-sizing','align-items','border-radius','object-fit','overflow-y','scrollbar-gutter'];
 return Object.fromEntries(selectors.map(selector=>{const e=document.querySelector(selector);if(!e)return[selector,null];const s=getComputedStyle(e),r=e.getBoundingClientRect();return[selector,{rect:{x:r.x,y:r.y,width:r.width,height:r.height},computed:Object.fromEntries(properties.map(k=>[k,s.getPropertyValue(k)])),parent:e.parentElement?.className,html:e.outerHTML.slice(0,1200)}];}));});await p.close();}
 await writeFile('.trellis/tasks/10-28-demo-fullsite-audit/evidence/visual/layout-first.json',JSON.stringify(result,null,2)+'\n');
 for(const k of ['reference','actual'])for(const [selector,v] of Object.entries(result[k]))if(v)console.log(k,selector,JSON.stringify(v.computed),JSON.stringify(v.rect));
}finally{await b.close();}

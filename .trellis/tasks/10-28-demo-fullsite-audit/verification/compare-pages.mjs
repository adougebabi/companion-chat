// User-authorized local Playwright. One isolated Chrome version/context setup
// for both immutable Demo and production dist, with no user browser profile.
import {createRequire} from 'node:module';
import {mkdir,readFile,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {primeReferenceWearing} from './reference-scene.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core'),{PNG}=require('pngjs');
const pixelmatch=(await import(require.resolve('pixelmatch'))).default;
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/visual');
const phase=process.env.AUDIT_CAPTURE_PHASE||'before';
const referenceBase=process.env.AUDIT_REFERENCE_BASE||'http://127.0.0.1:4179',actualBase=process.env.AUDIT_ACTUAL_BASE||'http://127.0.0.1:4381';
const selected=(process.argv[2]||'library,chat,now,schedule,wardrobe').split(',');
const referenceRoutes={media:'/media',visual:'/media?tab=identity',generation:'/media?tab=generate',avatar:'/media?tab=avatar'};
const routes={visual:'/c/linwan/media?tab=identity',generation:'/c/linwan/media?tab=generate',avatar:'/c/linwan/media?tab=avatar',library:'/library',chat:'/c/linwan/chat',now:'/c/linwan/life/now',schedule:'/c/linwan/life/schedule',wardrobe:'/c/linwan/life/wardrobe',goals:'/c/linwan/life/goals',diary:'/c/linwan/life/diary',memory:'/c/linwan/growth',profile:'/c/linwan/profile',world:'/c/linwan/world',people:'/c/linwan/people',feed:'/feed',media:'/c/linwan/media',studio:'/studio',inbox:'/inbox',settings:'/settings',creation:'/library/create',auth:'/auth'};
const browser=await chromium.launch({channel:'chrome',headless:true});
const results=[];
try{
 for(const size of [{name:'desktop',width:1348,height:925},{name:'mobile',width:390,height:844}]){
  const context=await browser.newContext({viewport:{width:size.width,height:size.height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',colorScheme:'light',reducedMotion:'reduce'});
  const reference=await context.newPage(),actual=await context.newPage();
  const referenceScene=selected.some(name=>['chat','now','wardrobe'].includes(name))?await primeReferenceWearing(reference,{base:referenceBase}):[];
  for(const name of selected){
   const out=resolve(root,name,phase);await mkdir(out,{recursive:true});
   const record={page:name,size,phase,referenceScene,browser:browser.version(),referenceUrl:referenceBase+'/fluctlight-offline.html#'+(referenceRoutes[name]||routes[name]),actualUrl:actualBase+'/#'+routes[name],errors:{reference:[],actual:[]}};
   const inspect=async(page,url,kind)=>{
    const listener=error=>record.errors[kind].push(error.message);page.on('pageerror',listener);
    // Demo media is an inline responsive SVG. Its HTTP test image must have
    // the same viewport as the actual image element, retaining the original
    // MediaImage/Art drawing and CSS rather than the browser's 300x150 default.
    // This does not resize the element or change reference pixels.
    const mediaPattern='**/api/media/parity-media-*';
    if(kind==='actual'&&name==='media')await page.route(mediaPattern,async route=>{
     const response=await route.fetch(),assetPath=new URL(route.request().url()).pathname;
     const frame=await page.evaluate(path=>{const image=Array.from(document.images).find(i=>new URL(i.src).pathname===path&&i.getBoundingClientRect().width>0);if(!image)return null;const r=image.getBoundingClientRect();return {width:r.width,height:r.height};},assetPath);
     let body=await response.text();
     if(frame?.width>0&&frame?.height>0){body=body.replace('<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%">','<svg xmlns="http://www.w3.org/2000/svg" width="'+frame.width+'" height="'+frame.height+'">');(record.fixtureImageViewports||=[]).push({assetPath,...frame,source:'immutable Demo MediaImage/Art; own production image viewport'});}
     await route.fulfill({response,body});
    });
    await page.goto(url,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor({state:'visible'});
    if(kind==='actual'&&name==='library')await page.waitForFunction(()=>document.querySelectorAll('.library-chips').length===3&&document.querySelectorAll('.library-chips span').length>0);
    if(name==='chat')await page.getByText('给你看看窗边这个小角落。',{exact:false}).waitFor();
    await page.mouse.move(size.width-1,size.height-1);await page.evaluate(async()=>{window.scrollTo({top:0,left:0,behavior:'instant'});document.querySelector('.workspace-page')?.scrollTo({top:0,left:0,behavior:'instant'});await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));await Promise.all(Array.from(document.images).map(image=>image.complete?Promise.resolve():new Promise(resolve=>{image.addEventListener('load',resolve,{once:true});image.addEventListener('error',resolve,{once:true});})));});
    if(name==='chat')await page.locator('.companion-message-list').evaluate(e=>e.scrollTo({top:e.scrollHeight,behavior:'instant'}));
    record[kind]=await page.evaluate(()=>{
     const shape=element=>{const r=element.getBoundingClientRect(),s=getComputedStyle(element);return {tag:element.tagName,class:element.className,text:element.textContent?.trim().slice(0,140),x:r.x,y:r.y,width:r.width,height:r.height,font:s.fontFamily,fontSize:s.fontSize,fontWeight:s.fontWeight,lineHeight:s.lineHeight,color:s.color,padding:s.padding,gap:s.gap,display:s.display};};
     const first=selectors=>selectors.map(s=>document.querySelector(s)).find(Boolean);
     return {environment:{width:innerWidth,height:innerHeight,dpr:devicePixelRatio,scale:visualViewport.scale,timezone:Intl.DateTimeFormat().resolvedOptions().timeZone,windowScrollY:scrollY,font:getComputedStyle(document.body).fontFamily,documentWidth:document.documentElement.scrollWidth},heading:shape(document.querySelector('h1')),header:first(['.page-heading','.replica-page-heading'])?shape(first(['.page-heading','.replica-page-heading'])):null,landmarks:Array.from(document.querySelectorAll('aside,nav,header,.library-hero,.library-header,.tag-filter,.character-grid,.character-list,.library-card,.character-card,.companion-chat-panel,.companion-message-list,.companion-composer,.companion-moment-card,.lp-hero,.lp-grid')).map(shape),artRects:Array.from(document.querySelectorAll('.r-media-image-button,.media-card-art')).filter(e=>!e.parentElement?.closest('.r-media-image-button,.media-card-art')).map(shape),images:Array.from(document.images).map(i=>({alt:i.alt,src:i.getAttribute('src'),loaded:i.complete&&i.naturalWidth>0,width:i.width,height:i.height})),scrolling:Array.from(document.querySelectorAll('*')).filter(e=>e.clientHeight>0&&e.scrollHeight>e.clientHeight+1&&['auto','scroll'].includes(getComputedStyle(e).overflowY)).map(e=>({class:e.className,scrollTop:e.scrollTop,clientHeight:e.clientHeight,scrollHeight:e.scrollHeight})),buttons:Array.from(document.querySelectorAll('button')).filter(e=>e.getBoundingClientRect().width&&e.getBoundingClientRect().height).map(shape)};
    });
    await page.screenshot({path:resolve(out,kind+'-'+size.name+'.png'),fullPage:false,animations:'disabled'});page.off('pageerror',listener);if(kind==='actual'&&name==='media')await page.unroute(mediaPattern);
   };
   try{
    await inspect(reference,record.referenceUrl,'reference');await inspect(actual,record.actualUrl,'actual');
    const a=PNG.sync.read(await readFile(resolve(out,'reference-'+size.name+'.png'))),b=PNG.sync.read(await readFile(resolve(out,'actual-'+size.name+'.png'))),diff=new PNG({width:a.width,height:a.height});
    if(name==='media')record.artAssetDifferences=record.reference.artRects.map((r,i)=>{
     const other=record.actual.artRects[i];if(!other)return {index:i,missing:true};const width=Math.min(Math.floor(r.width),Math.floor(other.width)),height=Math.min(Math.floor(r.height),Math.floor(other.height));
     if(r.y<0||other.y<0||r.y+height>a.height||other.y+height>b.height)return {index:i,scope:'outside captured viewport; not measured'};
     const left=new PNG({width,height}),right=new PNG({width,height});for(let y=0;y<height;y++){a.data.copy(left.data,y*width*4,((Math.round(r.y)+y)*a.width+Math.round(r.x))*4,((Math.round(r.y)+y)*a.width+Math.round(r.x)+width)*4);b.data.copy(right.data,y*width*4,((Math.round(other.y)+y)*b.width+Math.round(other.x))*4,((Math.round(other.y)+y)*b.width+Math.round(other.x)+width)*4);}return {index:i,width,height,reference:{x:r.x,y:r.y,width:r.width,height:r.height},actual:{x:other.x,y:other.y,width:other.width,height:other.height},pixels:pixelmatch(left.data,right.data,null,width,height),scope:'aligned original media art only; full-page raw difference is still recorded separately'};
    });
    record.pixelDifference={pixels:pixelmatch(a.data,b.data,diff.data,a.width,a.height),total:a.width*a.height,meaning:'raw image difference only; no pass threshold, masking or completion inference'};
    await writeFile(resolve(out,'difference-'+size.name+'.png'),PNG.sync.write(diff));
    const pair=new PNG({width:a.width*2,height:a.height});for(let y=0;y<a.height;y++){a.data.copy(pair.data,y*pair.width*4,y*a.width*4,(y+1)*a.width*4);b.data.copy(pair.data,y*pair.width*4+a.width*4,y*b.width*4,(y+1)*b.width*4);}await writeFile(resolve(out,'comparison-'+size.name+'.png'),PNG.sync.write(pair));
    console.log(name,size.name,'captured; heading',record.reference.heading.x,record.reference.heading.y,'vs',record.actual.heading.x,record.actual.heading.y,'errors',record.errors.actual.length);
   }catch(error){record.failure=error.message;console.log(name,size.name,'FAILED',error.message.slice(0,180));}
   results.push(record);await writeFile(resolve(root,'capture-'+phase+'.json'),JSON.stringify(results,null,2)+'\n');
  }
  await context.close();
 }
}finally{await browser.close();}

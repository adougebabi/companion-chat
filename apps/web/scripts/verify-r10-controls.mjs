// Production browser audit. Screenshots, captured live GETs and CDP evidence stay in /tmp.
// live: fresh authenticated browser, GET/HEAD plus the login POST only.
// stale: inspect an existing 13001 tab through CDP without reloading its old assets.
// fixed: serve dist with captured GET responses; no upstream, Core or database access.
// PLAYWRIGHT_MODULE=/tmp/r8-browser/node_modules/playwright-core/index.mjs node apps/web/scripts/verify-r10-controls.mjs live|stale|fixed
import assert from 'node:assert/strict';
import http from 'node:http';
import {mkdir,readFile,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {resolve,extname} from 'node:path';
import {fileURLToPath} from 'node:url';

const root=fileURLToPath(new URL('../../../',import.meta.url));
const output='/tmp/r10-shots';
const properties=new Set(['width','height','min-width','min-height','padding','padding-top','padding-right','padding-bottom','padding-left','position','overflow','overflow-x','overflow-y','pointer-events','content','inset','inset-inline','inset-block','left','top','right','bottom','transform','background-color','box-shadow','outline','outline-width','outline-style','outline-color','outline-offset','--tw-ring-shadow','--tw-ring-color','--tw-ring-offset-width']);

export function verifyMeasurements(rows) {
  for(const row of rows){
    const label=`${row.viewport} ${row.path} ${row.name}`;
    if(row.kind==='focus'){
      assert.equal(row.focusVisible,true,label+' must be reached by real Tab');
      assert.notEqual(row.shadow,'none',label+' ring must survive the cascade');
      assert.match(row.shadow,/3px/,label+' computed shadow must include the 3px ring');
      assert.match(row.ringShadow,/3px/,label+' uses the primitive 3px ring');
    }
    if(row.size==='icon'){
      assert.deepEqual([row.rect.width,row.rect.height],[36,36],label+' visual size');
      assert.deepEqual(row.padding,['0px','0px','0px','0px'],label+' padding');
    }
    if(row.kind==='radio'||row.kind==='checkbox')assert.deepEqual([row.rect.width,row.rect.height],[16,16],label+' retains original visual size');
    if(row.kind==='filter')assert.equal(row.rect.height,28.5,label+' retains compact visual height');
    if(row.viewport==='mobile'&&row.kind!=='focus'){
      assert.equal(row.after.content,'""',label+' generated hit plane');
      assert.equal(row.after.background,'rgba(0, 0, 0, 0)',label+' transparent hit plane');
      assert.ok(parseFloat(row.after.width)>=44&&parseFloat(row.after.height)>=44,label+' minimum 44px hit plane');
      const directions=row.name==='全部'||row.name==='重新读取通知'?['top','bottom','left','right']:row.kind==='filter'||row.kind==='radio'||row.kind==='checkbox'?['top','bottom']:[];
      for(const direction of directions){
        assert.equal(row.hits[direction].hit,true,label+' elementFromPoint 10px '+direction);
        if(row.clicks)assert.equal(row.clicks[direction],true,label+' actual pointer target 10px '+direction);
      }
    }
  }
}

// CDP supplies matched rules, their layers and selector specificity. Preserve the
// full response as well as the winning declarations for the audited properties.
function cascade(matches,inline,headers){
  const rules=matches.map((match,order)=>{
    const rule=match.rule;
    const specificity=match.matchingSelectors.map(i=>rule.selectorList.selectors[i]?.specificity||{a:0,b:0,c:0}).sort((a,b)=>b.a-a.a||b.b-a.b||b.c-a.c)[0]||{a:0,b:0,c:0};
    return {selector:rule.selectorList.text,origin:rule.origin,source:headers.get(rule.styleSheetId)?.sourceURL||'',line:rule.style.range?.startLine,layers:rule.layers?.map(l=>l.text)||[],specificity,order,declarations:rule.style.cssProperties.filter(p=>properties.has(p.name)&&!p.disabled&&p.parsedOk!==false)};
  });
  if(inline?.cssProperties?.length)rules.push({selector:'element.style',origin:'regular',layers:[],specificity:{a:1000000,b:0,c:0},order:rules.length,declarations:inline.cssProperties.filter(p=>properties.has(p.name)&&!p.disabled&&p.parsedOk!==false)});
  const winner={};
  const layerOrder={theme:0,base:1,components:2,utilities:3};
  const rank=(rule,p,index)=>{
    const layered=rule.layers.length>0,layer=layerOrder[rule.layers.at(-1)]??0;
    return [p.important?1:0,rule.origin==='user-agent'?0:1,p.important?(layered?10-layer:0):(layered?layer:10),rule.specificity.a,rule.specificity.b,rule.specificity.c,rule.order,index];
  };
  const greater=(a,b)=>{for(let i=0;i<a.length;i++)if(a[i]!==b[i])return a[i]>b[i];return true;};
  for(const rule of rules)for(const [index,p]of rule.declarations.entries()){
    const score=rank(rule,p,index);
    if(!winner[p.name]||greater(score,winner[p.name].score))winner[p.name]={value:p.value,selector:rule.selector,layers:rule.layers,source:rule.source,important:!!p.important,score};
  }
  return {rules,winners:Object.fromEntries(Object.entries(winner).map(([name,{score,...value}])=>[name,value]))};
}

async function settle(page){
  await page.evaluate(async()=>{
    await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a=>a.finished.catch(()=>{})));
    await new Promise(done=>requestAnimationFrame(()=>requestAnimationFrame(done)));
  });
}

async function tabTo(page,element){
  for(let i=0;i<120;i++){
    await page.keyboard.press('Tab');
    if(await element.evaluate(e=>e===document.activeElement))return;
  }
  assert.fail('Real Tab did not reach '+await element.textContent());
}

async function measure(page,cdp,headers,selector,index,{path,viewport,kind,clicks}){
  const element=page.locator(selector).nth(index);
  await element.scrollIntoViewIfNeeded();
  if(kind==='focus')await tabTo(page,element);
  await settle(page);
  const value=await element.evaluate(e=>{
    const r=e.getBoundingClientRect(),s=getComputedStyle(e),a=getComputedStyle(e,'::after');
    const hits={};
    for(const[direction,x,y]of[['top',r.x+r.width/2,r.y-10],['bottom',r.x+r.width/2,r.bottom+10],['left',r.x-10,r.y+r.height/2],['right',r.right+10,r.y+r.height/2]]){
      const hit=document.elementFromPoint(x,y);
      hits[direction]={x,y,hit:hit===e||e.contains(hit),target:hit?.outerHTML.slice(0,240)};
    }
    const ancestors=[];
    for(let p=e;p;p=p.parentElement){const style=getComputedStyle(p),box=p.getBoundingClientRect();ancestors.push({tag:p.tagName,className:p.className,rect:{x:box.x,y:box.y,width:box.width,height:box.height},position:style.position,overflowX:style.overflowX,overflowY:style.overflowY,pointerEvents:style.pointerEvents});}
    return {name:e.getAttribute('aria-label')||e.textContent.trim(),tag:e.tagName,slot:e.dataset.slot,size:e.dataset.size,variant:e.dataset.variant,className:e.className,rect:{x:r.x,y:r.y,width:r.width,height:r.height},focusVisible:e.matches(':focus-visible'),shadow:s.boxShadow,outline:s.outline,ringShadow:s.getPropertyValue('--tw-ring-shadow'),ringColor:s.getPropertyValue('--tw-ring-color'),ringOffset:s.getPropertyValue('--tw-ring-offset-width'),position:s.position,padding:[s.paddingTop,s.paddingRight,s.paddingBottom,s.paddingLeft],after:{content:a.content,width:a.width,height:a.height,left:a.left,top:a.top,position:a.position,pointerEvents:a.pointerEvents,background:a.backgroundColor},ancestors,hits};
  });
  await cdp.send('DOM.getDocument');
  const{result}=await cdp.send('Runtime.evaluate',{expression:`document.querySelectorAll(${JSON.stringify(selector)})[${index}]`});
  const{nodeId}=await cdp.send('DOM.requestNode',{objectId:result.objectId});
  const matched=await cdp.send('CSS.getMatchedStylesForNode',{nodeId});
  await cdp.send('Runtime.releaseObject',{objectId:result.objectId});
  const evidence={...cascade(matched.matchedCSSRules||[],matched.inlineStyle,headers),pseudo:(matched.pseudoElements||[]).map(p=>({type:p.pseudoType,...cascade(p.matches||[],null,headers)}))};
  if(clicks&&viewport==='mobile'&&kind!=='focus'){
    value.clicks={};
    const directions=value.name==='全部'||value.name==='重新读取通知'?['top','bottom','left','right']:kind==='filter'||kind==='radio'||kind==='checkbox'?['top','bottom']:[];
    for(const direction of directions){
      // Capture actual pointer dispatch while preventing forms, navigation and
      // domain handlers. This is used only on the isolated local server.
      await element.evaluate(e=>{window.__r10PointerHit=false;document.addEventListener('click',event=>{window.__r10PointerHit=event.target===e||e.contains(event.target);event.preventDefault();event.stopImmediatePropagation();},{capture:true,once:true});});
      await page.mouse.click(value.hits[direction].x,value.hits[direction].y);
      value.clicks[direction]=await page.evaluate(()=>window.__r10PointerHit);
    }
    if(kind==='checkbox'||value.name==='使用日常生活世界'){
      value.activation=[];
      for(const direction of ['top','bottom']){
        const before=await element.getAttribute('aria-checked');
        await page.mouse.click(value.hits[direction].x,value.hits[direction].y);
        const expected=kind==='checkbox'?(before==='true'?'false':'true'):'true';
        await page.waitForFunction(({selector,index,expected})=>document.querySelectorAll(selector)[index]?.getAttribute('aria-checked')===expected,{selector,index,expected});
        const box=await element.boundingBox();
        const noHitPlane=await page.addStyleTag({content:'[data-slot="radio-group-item"]::after,[data-slot="checkbox"]::after{content:none!important}'});
        const originalVisual=await element.boundingBox();
        await noHitPlane.evaluate(e=>e.remove());
        assert.deepEqual([box.width,box.height],[originalVisual.width,originalVisual.height],'10px hit plane preserves the original visual geometry in the same selected state');
        value.activation.push({direction,before,after:await element.getAttribute('aria-checked'),visual:[box.width,box.height],withoutHitPlane:[originalVisual.width,originalVisual.height]});
      }
    }
    if(value.name==='重新读取通知'){
      await Promise.all([
        page.waitForRequest(request=>request.method()==='GET'&&new URL(request.url()).pathname==='/api/notifications'),
        page.mouse.click(value.hits.top.x,value.hits.top.y),
      ]);
      value.activation='10px top click performed the original notifications GET';
      await page.waitForFunction(()=>!document.querySelector('[aria-label="重新读取通知"]')?.disabled);
    }
  }
  return {...value,path,viewport,kind,selector,index,cascade:evidence,matched};
}

async function replayServer(){
  const snapshot=JSON.parse(await readFile(output+'/snapshot.json','utf8'));
  const exact=new Map(snapshot.map(r=>[r.path,r])),byPath=new Map(snapshot.map(r=>[new URL(r.path,'http://fixture').pathname,r]));
  const missing=[];
  const server=http.createServer(async(req,res)=>{
    try{
      if(!['GET','HEAD'].includes(req.method)){res.writeHead(405);return res.end();}
      const url=new URL(req.url,'http://fixture'),path=url.pathname;
      if(path==='/runtime-config.js'){res.writeHead(200,{'content-type':'text/javascript'});return res.end('window.__FLUCTLIGHT_RUNTIME_CONFIG__={apiOrigin:location.origin};');}
      if(path.startsWith('/api/')||path.startsWith('/auth/')){
        const row=exact.get(path+url.search)||byPath.get(path);
        if(!row){missing.push(path+url.search);res.writeHead(404);return res.end('{}');}
        res.writeHead(row.status,{'content-type':row.headers['content-type']||'application/json','cache-control':'no-store'});return res.end(Buffer.from(row.body,'base64'));
      }
      const file=resolve(root,'apps/web/dist','.'+(path==='/'?'/index.html':path));
      assert.ok(file.startsWith(resolve(root,'apps/web/dist')+'/'));
      const body=await readFile(file),mime={'.html':'text/html','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml'}[extname(file)]||'application/octet-stream';
      res.writeHead(200,{'content-type':mime,'cache-control':'no-cache'});res.end(body);
    }catch{res.writeHead(404);res.end();}
  });
  await new Promise((ok,no)=>{server.once('error',no);server.listen(0,'127.0.0.1',ok);});
  return {base:`http://127.0.0.1:${server.address().port}`,missing,stop:()=>new Promise(ok=>server.close(ok))};
}

export async function run(mode){
  assert.ok(['live','stale','fixed'].includes(mode));
  await mkdir(output,{recursive:true});
  const{chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright-core');
  const local=mode==='fixed'?await replayServer():null,base=local?.base||'http://127.0.0.1:13001';
  const browser=mode==='stale'?await chromium.connectOverCDP(process.env.R10_CDP||'http://127.0.0.1:9233'):await chromium.launch({executablePath:process.env.CHROME_PATH||'/opt/google/chrome/chrome',headless:true,args:['--no-sandbox']});
  let context,page,cdp;
  const snapshots=[],assets=[],pending=[],blocked=[],errors=[],rows=[],headers=new Map();
  const guard=route=>{
    const req=route.request(),url=new URL(req.url());
    if(!['data:','blob:'].includes(url.protocol)&&url.origin!==base)return route.abort();
    if(!['GET','HEAD'].includes(req.method())&&!(mode==='live'&&req.method()==='POST'&&url.pathname==='/auth/login')){blocked.push(req.method()+' '+url.pathname);return route.abort();}
    return route.continue();
  };
  try{
    context=mode==='stale'?browser.contexts().find(c=>c.pages().some(p=>p.url().startsWith(base))):await browser.newContext({viewport:{width:1440,height:1000},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
    assert.ok(context,'A deployed app tab must already exist for stale mode');
    page=mode==='stale'?context.pages().find(p=>p.url().startsWith(base)):await context.newPage();
    page.setDefaultTimeout(12000);
    await page.route('**/*',guard);
    if(mode==='live')await context.addInitScript(()=>{
      // Allow transient view state without saving a draft into browser storage.
      const overlay=new Map(),get=Storage.prototype.getItem,set=Storage.prototype.setItem,remove=Storage.prototype.removeItem;
      Storage.prototype.getItem=function(key){return this===localStorage&&overlay.has(String(key))?overlay.get(String(key)):get.call(this,key);};
      Storage.prototype.setItem=function(key,value){if(this===localStorage)overlay.set(String(key),String(value));else set.call(this,key,value);};
      Storage.prototype.removeItem=function(key){if(this===localStorage)overlay.set(String(key),null);else remove.call(this,key);};
    });
    page.on('pageerror',e=>errors.push(e.message));
    page.on('response',response=>{
      const url=new URL(response.url()),type=response.headers()['content-type']||'';
      if(response.request().method()!=='GET')return;
      if(!url.pathname.endsWith('.css')&&!url.pathname.endsWith('.js')&&!type.includes('application/json'))return;
      pending.push((async()=>{try{
        const body=await response.body();
        if(url.pathname.startsWith('/assets/'))assets.push({path:url.pathname,sha256:createHash('sha256').update(body).digest('hex'),cache:response.headers()['cache-control']});
        else if(mode==='live'&&(url.pathname.startsWith('/api/')||url.pathname==='/auth/session'))snapshots.push({path:url.pathname+url.search,status:response.status(),headers:{'content-type':type},body:body.toString('base64')});
      }catch{}})());
    });
    cdp=await context.newCDPSession(page);
    cdp.on('CSS.styleSheetAdded',({header})=>headers.set(header.styleSheetId,header));
    await cdp.send('DOM.enable');await cdp.send('CSS.enable');
    if(mode!=='stale'){
      await page.goto(base,{waitUntil:'domcontentloaded'});
      if(mode==='live'){
        await page.locator('#owner-password').waitFor();
        await page.locator('#owner-password').fill((await readFile('/tmp/ft-owner-pw.txt','utf8')).trim());
        await page.locator('button[type=submit]').click();
      }
      await page.getByRole('button',{name:'查看通知',exact:true}).waitFor();
    }
    const open=async path=>{
      await page.evaluate(path=>{location.hash='#/'+path;},path);
      const ready=path==='library/create'?'[aria-label="使用日常生活世界"]':path==='library'?'.tag-filter>button':path==='worlds'||path==='studio'?'[aria-label="刷新创作资源"]':path==='media'?'[aria-label="查询媒体"]':'[aria-label="重新读取通知"]';
      await page.locator(ready).waitFor();
      if(path==='library/create'){
        const tab=page.getByRole('tab',{name:'日常生活世界',exact:true});
        if(await tab.count())await tab.click();
      }
      if(path==='worlds'||path==='studio')await page.waitForFunction(()=>!document.querySelector('[aria-label="刷新创作资源"]')?.disabled);
      await settle(page);
    };
    // Read-only wizard navigation supplies the same transient browser draft as
    // the audit's existing tab, allowing its exact "继续创建" link to be measured.
    await open('library/create');
    for(const[viewport,width,height]of[['desktop',1440,1000],['mobile',390,844]]){
      await page.setViewportSize({width,height});
      for(const path of ['library','worlds','studio','media','inbox','library/create']){
        await open(path);
        let targets=[];
        if(path==='library'){
          await page.getByRole('link',{name:'先连接模型',exact:true}).waitFor();
          targets=[['a[data-variant="outline"]','focus'],['.tag-filter>button','filter']];
        }
        if(path==='worlds'||path==='studio')targets=[['[aria-label="刷新创作资源"]','icon'],['[aria-label="查询"]','icon'],['[aria-label="已归档资源"]','checkbox']];
        if(path==='media')targets=[['[aria-label="查询媒体"]','icon']];
        if(path==='inbox')targets=[['[aria-label="重新读取通知"]','icon']];
        if(path==='library/create')targets=[['[role="radio"]','radio']];
        targets.push(['[aria-label="查看通知"]','icon']);
        for(const[selector,kind]of targets)for(let i=0;i<await page.locator(selector).count();i++){
          if(!await page.locator(selector).nth(i).isVisible())continue;
          const row=await measure(page,cdp,headers,selector,i,{path,viewport,kind,clicks:mode==='fixed'});
          const{matched,...compact}=row;rows.push(compact);
          await writeFile(`${output}/${mode}-cdp-${viewport}-${path.replaceAll('/','-')}-${rows.length}.json`,JSON.stringify(matched,null,2));
          if(kind==='focus')await page.screenshot({path:`${output}/${mode}-${viewport}-focus-${row.name}.png`,fullPage:true});
        }
        await page.screenshot({path:`${output}/${mode}-${viewport}-${path.replaceAll('/','-')}.png`,fullPage:true});
      }
    }
    await Promise.race([Promise.allSettled(pending),new Promise(ok=>setTimeout(ok,3000))]);
    if(mode==='live')await writeFile(output+'/snapshot.json',JSON.stringify(snapshots));
    const styles=[];
    for(const[id,header]of headers)if(header.sourceURL.includes('/assets/')){
      try{const{text}=await cdp.send('CSS.getStyleSheetText',{styleSheetId:id});styles.push({url:header.sourceURL,sha256:createHash('sha256').update(text).digest('hex')});}catch{}
    }
    const documentAssets=await page.evaluate(()=>({scripts:[...document.scripts].map(s=>s.src).filter(Boolean),styles:[...document.styleSheets].map(s=>s.href).filter(Boolean)}));
    const report={mode,base,boundary:mode==='fixed'?'local production dist + isolated replay of deployed GETs; no database or upstream':'read-only deployed app, login POST only; transient Chrome state',documentAssets,styles,assets,rows,blocked,errors,missing:local?.missing||[]};
    await writeFile(`${output}/${mode}-measurements.json`,JSON.stringify(report,null,2));
    if(mode==='fixed'){
      assert.deepEqual(blocked,[]);assert.deepEqual(errors,[]);assert.deepEqual(local.missing,[]);
      for(const viewport of ['desktop','mobile'])for(const name of ['继续创建','先连接模型'])assert.ok(rows.some(r=>r.viewport===viewport&&r.name===name),'exact home link required: '+name);
      verifyMeasurements(rows);
    }
    console.log(JSON.stringify({mode,targets:rows.length,styles,blocked,errors,missing:local?.missing||[]}));
  }catch(error){
    if(page&&!page.isClosed()){await page.screenshot({path:`${output}/${mode}-failure.png`,fullPage:true});await writeFile(`${output}/${mode}-failure.txt`,await page.locator('body').innerText());}
    throw error;
  }finally{
    await cdp?.detach();
    if(mode==='stale')await page?.unroute('**/*',guard);
    await browser.close();await local?.stop();
  }
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))await run(process.argv[2]||'fixed');

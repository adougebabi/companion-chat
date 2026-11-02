import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {startOwnedFixture} from '../10-28-demo-fullsite-audit/verification/owned-fixture.mjs';
import {chromium} from '/tmp/r22-browser/node_modules/playwright-core/index.mjs';

const fixture=await startOwnedFixture({port:4284,env:{FLUCTLIGHT_UI_FIXTURE_MANAGEMENT:'normal'}});
const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
const shots='/tmp/r4-ui-shots',results=[];await mkdir(shots,{recursive:true});
try{
  for(const [size,width,height] of [['desktop',1920,1113],['mobile',390,844]]){
    const context=await browser.newContext({viewport:{width,height},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage(),errors=[];
    page.setDefaultTimeout(10000);page.on('pageerror',error=>errors.push(error.message));
    await page.route('**/api/fluctlights',route=>route.fulfill({json:[]}));
    await page.route(/\/api\/studio\/resources(?:\?|$)/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
    await page.route('**/api/configuration-catalog',async route=>{const response=await route.fetch(),config=await response.json();await route.fulfill({response,json:{...config,bindings:[]}});});
    await page.route(/\/api\/diagnostic-records\/workflow-intents(?:\?|$)/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
    const go=async path=>{await page.goto(fixture.base+'/#'+path,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
    const shot=async name=>{
      assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'no horizontal overflow: '+size+' '+name);
      await page.screenshot({path:shots+'/'+size+'-'+name+'.png',fullPage:true,animations:'disabled'});
    };
    await go('/library');await page.getByText('模型服务已保存，但尚未绑定通用模型用途。',{exact:false}).waitFor();
    const bind=page.getByRole('link',{name:'去绑定通用模型',exact:true});assert.equal(await bind.getAttribute('href'),'#/settings?section='+encodeURIComponent('模型用途'));await shot('library-unbound');
    await bind.click();await page.locator('[data-section="模型用途"]:visible').waitFor();results.push({size,item:'library-binding-link',status:'passed'});
    await go('/library/create');const daily=page.getByRole('radio',{name:'使用日常生活世界',exact:true}),create=page.getByRole('radio',{name:'创建新的世界底稿',exact:true});
    await daily.waitFor();assert.equal(await daily.getAttribute('aria-checked'),'false');assert.equal(await create.getAttribute('aria-checked'),'false');
    const cards=await page.locator('.creation-radio-card').evaluateAll(nodes=>nodes.map(node=>{const rect=node.getBoundingClientRect();return {x:rect.x,y:rect.y,width:rect.width,height:rect.height};}));
    assert.equal(cards.length,2);assert.ok(Math.abs(cards[0].width-cards[1].width)<2);assert.ok(Math.abs(cards[0].height-cards[1].height)<2);await shot('world-radio-unselected');
    await page.locator('label.creation-daily').click({position:{x:40,y:40}});assert.equal(await daily.getAttribute('aria-checked'),'true');assert.equal(await page.getByTestId('wizard-world-next').isEnabled(),true);await page.getByText('将为 TA 准备一个日常生活世界',{exact:true}).waitFor();
    await create.focus();await page.keyboard.press('Space');assert.equal(await create.getAttribute('aria-checked'),'true');assert.equal(await page.getByTestId('wizard-world-next').isDisabled(),true);
    await page.getByRole('link',{name:'去创作中心创建',exact:true}).waitFor();await shot('world-new-draft-return');
    const draft=await page.evaluate(()=>Object.keys(localStorage).filter(key=>key.includes('fluctlight-creation')).map(key=>[key,localStorage.getItem(key)]));
    await page.getByRole('link',{name:'去创作中心创建',exact:true}).click();await page.waitForURL('**/#/studio/edit/new?kind=world');
    await page.evaluate(()=>{location.hash='#/library';});await page.getByRole('link',{name:'继续创建',exact:true}).waitFor();
    await page.getByRole('link',{name:'继续创建',exact:true}).click();await daily.waitFor();assert.equal(await daily.getAttribute('aria-checked'),'true');
    assert.deepEqual(await page.evaluate(()=>Object.keys(localStorage).filter(key=>key.includes('fluctlight-creation')).map(key=>[key,localStorage.getItem(key)])),draft);
    results.push({size,item:'world-radio-and-return',cards,status:'passed'});
    await go('/worlds');const empty=page.locator('.studio-empty');await empty.getByRole('heading',{name:'还没有世界底稿',exact:true}).waitFor();assert.equal(await page.getByRole('button',{name:'创建世界底稿',exact:true}).count(),1);assert.equal(await empty.locator('[data-slot=button]').count(),1);await shot('worlds-empty');
    await page.getByRole('searchbox',{name:'搜索资源名称',exact:true}).fill('未匹配');await page.getByRole('button',{name:'查询',exact:true}).click();await empty.getByRole('heading',{name:'没有找到相关资源',exact:true}).waitFor();await page.getByRole('button',{name:'清除筛选',exact:true}).click();await empty.getByRole('heading',{name:'还没有世界底稿',exact:true}).waitFor();results.push({size,item:'world-empty-and-filter',status:'passed'});
    await go('/diag?view=tasks');await page.getByRole('heading',{name:'暂时没有运行任务',exact:true}).waitFor();const next=page.getByRole('link',{name:'去创建角色',exact:true});assert.equal(await next.getAttribute('href'),'#/library/create');await shot('diagnostic-next-step');await next.click();await daily.waitFor();results.push({size,item:'diagnostic-next-step',status:'passed'});
    assert.deepEqual(errors,[]);await context.close();
  }
  const context=await browser.newContext(),page=await context.newPage();page.setDefaultTimeout(10000);let documents=0,chunks=0;
  page.on('request',request=>{if(request.isNavigationRequest()&&request.frame()===page.mainFrame())documents++;});
  await page.route('**/auth/session',route=>route.fulfill({json:{authenticated:false}}));
  const chunkPattern=/\/assets\/AuthPage-[^/]+\.js$/;
  await page.route(chunkPattern,route=>{chunks++;return route.fulfill({status:404,contentType:'text/javascript',body:'/* stale deployed chunk */'});});
  await page.goto(fixture.base+'/#/auth',{waitUntil:'networkidle'}).catch(error=>{if(!error.message.includes('interrupted by another navigation'))throw error;});
  await page.getByText('页面资源未能加载',{exact:true}).waitFor();await page.waitForLoadState('networkidle');
  assert.equal(documents,2,'exactly one automatic reload');assert.equal(chunks,2);assert.equal(await page.evaluate(()=>sessionStorage.getItem('fluctlight:chunk-reload-attempted')),'1');
  await page.screenshot({path:shots+'/auth-stale-chunk-prompt.png'});
  await page.unroute(chunkPattern);await page.getByRole('button',{name:'重新加载页面',exact:true}).click();await page.getByRole('button',{name:'登录',exact:true}).waitFor();assert.equal(documents,3);assert.equal(await page.getByText('页面资源未能加载',{exact:true}).count(),0);results.push({item:'AuthPage-404-recovery',automaticReloads:1,manualReloadRecovered:true,status:'passed'});await context.close();
  await writeFile(new URL('./browser-results.json',import.meta.url),JSON.stringify({status:'passed',boundary:'production dist + isolated HTTP fixture; nginx cache policy covered by targeted configuration test',shots,results},null,2)+'\n');
  console.log('PASS R4 targeted browser checks: '+results.length+' records; '+shots);
}finally{await browser.close();await fixture.stop();}

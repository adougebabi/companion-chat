import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {startOwnedFixture} from '../10-28-demo-fullsite-audit/verification/owned-fixture.mjs';
import {chromium} from '/tmp/r22-browser/node_modules/playwright-core/index.mjs';

const fixture=await startOwnedFixture({port:4294,env:{FLUCTLIGHT_UI_FIXTURE_WORLD:'normal',FLUCTLIGHT_UI_FIXTURE_STUDIO:'normal',FLUCTLIGHT_UI_FIXTURE_WARDROBE_WORLD:'1',FLUCTLIGHT_UI_FIXTURE_MANAGEMENT:'normal',FLUCTLIGHT_UI_FIXTURE_GROWTH:'normal'}});
const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
const shots='/tmp/r34c-shots',results=[],fid='fluctlight-fixture-one',worldId=fid+'-world-one';
await mkdir(shots,{recursive:true});
const actual='2026-11-02T12:39:00Z',old='2026-11-02T10:00:00Z';
const taskData={avatar:{task_id:'avatar',kind:'visual_identity.prepare',title:'人物形象准备',fluctlight_id:fid,status:'running',created_at:old},schedule:{task_id:'schedule',kind:'schedule.current_day',title:'今日日程准备',fluctlight_id:fid,status:'running',created_at:old},older:{task_id:'older',kind:'wardrobe.initialize',status:'failed',fluctlight_id:fid,created_at:old},paused:{task_id:'paused',kind:'persona.compile',status:'paused',fluctlight_id:fid,created_at:old}};
const clock={task_id:'clock',intent_id:'clock',intent_type:'world.events.generate',status:'waiting',world_id:worldId,world_name:'温和魔法世界',created_at:actual};
const pendingRows=['older','paused'].map(id=>({id:'workflow:'+id,source_id:id,title:taskData[id].kind,status:taskData[id].status,fluctlight_id:fid,href:'/diag/task/'+id,created_at:old}));
try{
 for(const [size,width,height] of [['desktop',1440,1000],['mobile',390,844]]){
  const context=await browser.newContext({viewport:{width,height},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage(),errors=[];
  page.setDefaultTimeout(12000);page.on('pageerror',error=>errors.push(error.message));
  await page.route(/\/api\/pending(?:\?|$)/,route=>{const kind=new URL(route.request().url()).searchParams.get('kind');return route.fulfill({json:{groups:(kind?[kind]:['capability','persona_proposal','studio_apply','failed_task','owner_request']).map(kind=>({kind,status:'ok',count:kind==='failed_task'?2:0,items:kind==='failed_task'?pendingRows:[],next_cursor:''}))}});});
  await page.route(/\/api\/diagnostics\/tasks\/[^/?]+/,route=>route.fulfill({json:taskData[decodeURIComponent(new URL(route.request().url()).pathname.split('/').at(-1))]||{task_id:'clock',kind:'world.events.generate',status:'waiting',source:{world_id:worldId}}}));
  await page.route(/\/api\/diagnostic-records\/workflow-intents(?:\?|$)/,route=>{const query=new URL(route.request().url()).searchParams;const rows=[clock,taskData.avatar,taskData.schedule,{task_id:'obsolete',kind:'wardrobe.initialize',status:'failed',created_at:old}].filter(row=>(!query.get('intentType')||row.intent_type===query.get('intentType'))&&(!query.get('worldId')||row.world_id===query.get('worldId')));return route.fulfill({json:{items:rows.map(data=>({ref:data.task_id,occurred_at:data.created_at,data})),next_cursor:''}});});
  await page.route(/\/api\/diagnostic-records\/world-event-generations(?:\?|$)/,route=>route.fulfill({json:{items:[{ref:'generation',data:{world_id:new URL(route.request().url()).searchParams.get('worldId'),status:'no_event',slot_at:'2026-11-02T12:00:00Z',created_at:actual}}],next_cursor:''}}));
  await page.route(/\/api\/notifications(?:\?|$)/,route=>route.fulfill({json:{items:[{id:'named',title:'新回复',source_kind:'conversation.reply.published',fluctlight_id:fid,body:'今天过得怎样？',created_at:actual}],as_of:actual,next_cursor:''}}));
  await page.route(/\/api\/studio\/resources(?:\?|$)/,route=>route.fulfill({json:{items:[{id:'event',kind:'event_template',name:'午后阵雨',status:'active',published_version:1,revision:1},{id:'npc',kind:'npc',name:'茶馆老板',status:'draft',published_version:null,revision:1}],next_cursor:''}}));
  await page.route(/\/inventory\/preparation(?:\?|$)/,route=>route.fulfill({json:{fluctlight_id:fid,status:'not_started',can_recover:false}}));
  await page.route(/\/api\/fluctlights\/[^/]+\/memories(?:\?|$)/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
  const go=async path=>{await page.goto(fixture.base+'/#'+path,{waitUntil:'networkidle'});await page.locator('.workspace-page').waitFor();};
  const shot=async name=>{assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),size+' '+name+' horizontal overflow');await page.screenshot({path:`${shots}/${size}-${name}.png`,fullPage:true,animations:'disabled'});results.push({size,item:name,status:'passed'});};
  await go('/diag?view=tasks');await page.locator('[data-task-status-group=attention] strong').filter({hasText:'2'}).waitFor();
  assert.equal(await page.locator('.replica-task-row[data-task-status-group=attention]').count(),2);
  assert.equal(await page.locator('.replica-task-row').filter({hasText:'运行超过 15 分钟'}).count(),2);
  await page.getByText('20:00 整点（20:39 开始）',{exact:false}).waitFor();await shot('diagnostics-count-stale-clock');
  await page.locator('.replica-task-toolbar').scrollIntoViewIfNeeded();await shot('diagnostics-task-list');
  await page.getByRole('combobox',{name:'任务世界',exact:true}).click();await page.getByRole('option',{name:'温和魔法世界',exact:true}).click();assert.equal(await page.locator('.replica-task-row').count(),1);await shot('diagnostics-world-filter');
  await go('/inbox');await page.getByRole('heading',{name:'林岚 · 验收样本 · 新回复',exact:true}).waitFor();await page.locator('.replica-pending-group h3').filter({hasText:'服务异常'}).filter({hasText:'2'}).waitFor();await shot('inbox-name-count');
  assert.equal(await page.locator('.sr-only').filter({hasText:'Toggle Sidebar'}).count(),0);assert.equal(await page.locator('[aria-label="Toggle Sidebar"],[title="Toggle Sidebar"]').count(),0);
  await go('/studio');await page.locator('.r-studio-stat strong').filter({hasText:'2'}).waitFor();await page.getByText('已发布 1 · 草稿 1 · 已归档 0',{exact:true}).waitFor();await shot('studio-labelled-counts');
  await go('/c/'+fid+'/life/wardrobe');await page.getByText('已登记 9 件衣物，初始穿搭尚未开始',{exact:false}).waitFor();assert.equal(await page.locator('[aria-label="当前穿着的衣物"] li').count(),6);await shot('wardrobe-records-preparation');
  await go('/c/'+fid+'/growth');await page.getByRole('heading',{name:'还没有记录下记忆',exact:true}).waitFor();assert.equal(await page.getByRole('button',{name:'清除筛选',exact:true}).count(),0);await shot('memory-unfiltered-empty');
  await go('/library/create');await page.getByRole('heading',{name:'先为相遇，选一个世界。',exact:true}).waitFor();await page.locator('.creation-aside h3').filter({hasText:'待选择'}).waitFor({state:width>950?'visible':'attached'});assert.equal(await page.getByRole('radio',{checked:true}).count(),0);assert.equal(await page.locator('[data-testid=wizard-world-next]').isDisabled(),true);assert.equal(await page.getByText('将为 TA 准备一个日常生活世界',{exact:false}).count(),0);await shot('wizard-pending-world');
  await page.getByRole('radio').first().click();await page.locator('[data-testid=wizard-world-next]').click();await page.getByRole('heading',{name:'你想怎样认识这个人？',exact:true}).waitFor();assert.equal(await page.locator('.workspace-page').evaluate(node=>node.scrollTop),0);await shot('wizard-step2-top');
  await go('/c/'+fid+'/world');await page.getByRole('button',{name:'新建世界',exact:true}).waitFor();assert.equal(await page.getByRole('button',{name:'新建世界',exact:true}).count(),1);assert.equal(await page.locator('.r-create-world').count(),0);await shot('world-one-create-entry');
  await page.getByRole('searchbox',{name:'搜索世界名称或类型',exact:true}).fill('不存在的世界');await page.getByRole('heading',{name:'没有找到匹配的世界',exact:true}).waitFor();assert.equal(await page.locator('.r-create-world').count(),0);await shot('world-empty-search');
  await page.getByRole('button',{name:'清除搜索',exact:true}).click();await page.getByRole('button',{name:'进入世界 温和魔法世界',exact:true}).click();await page.getByRole('tab',{name:'世界事件',exact:false}).click();await page.getByText('20:00 整点（20:39 开始）',{exact:false}).first().waitFor();await page.getByRole('heading',{name:'自动世界事件',exact:true}).scrollIntoViewIfNeeded();await shot('world-clock-slot-and-start');
  await page.getByRole('button',{name:'全部世界',exact:true}).click();await page.getByRole('button',{name:'进入世界 远山森林',exact:true}).click();await page.getByRole('heading',{name:'远山森林',exact:true}).first().waitFor();assert.equal(await page.locator('.r-world-hero h2').textContent(),'远山森林');
  await page.getByRole('tab',{name:'世界设定',exact:false}).click();await page.getByRole('button',{name:'创建人物底稿',exact:true}).scrollIntoViewIfNeeded();await shot('unbound-world-people-guidance');
  await page.getByRole('tab',{name:'世界事件',exact:false}).click();await page.getByRole('button',{name:'创建事件底稿',exact:true}).scrollIntoViewIfNeeded();assert.ok((await page.getByText('今天 20:00 整点（20:39 开始）',{exact:true}).boundingBox()).width>=200,'world time description has a readable width');await shot('unbound-world-events-guidance');
  assert.deepEqual(errors,[]);await context.close();
 }
 await writeFile(new URL('./browser-results.json',import.meta.url),JSON.stringify({status:'passed',boundary:'local production dist + isolated loopback HTTP fixture; no real Core/Provider/DB writes',shots,results},null,2)+'\n');
 console.log(`PASS ${results.length} production Chrome screenshots in ${shots}`);
}finally{await browser.close();await fixture.stop();}

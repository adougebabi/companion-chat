import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {expectNoLayoutDefects} from './support/layout';
const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
test.beforeEach(async({page})=>{await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());});
test('imported pack names lead to the real editors while the complete mapping stays downloadable',{tag:'@data-closeout'},async({page})=>{
 const pack={id:'pack-closeout',digest:'digest-private-fixture',created_at:'2026-11-23T01:30:00Z',revision:3,resource_versions:[{name:'青岚小城的设定',kind:'world',entry_key:'entry-private',resource_id:'resource-private',version:7}],groups:[],preferences:{},publish_order:['entry-private'],external_bindings:[],future:{sample:[null,false,0]}};
 await page.route(/\/api\/studio\/packs\/pack-closeout$/,route=>route.fulfill({json:pack}));
 await page.goto(origin+'/#/data/pack/pack-closeout');
 const workspace=page.getByTestId('domain-workspace');await expect(workspace.getByText('青岚小城的设定',{exact:true})).toBeVisible();
 await expect(workspace.getByRole('link',{name:/青岚小城的设定/})).toHaveAttribute('href','#/studio/edit/resource-private');
 const visible=await workspace.innerText();for(const id of [pack.id,pack.digest,'entry-private','resource-private'])expect(visible).not.toContain(id);
 await expectNoLayoutDefects(page,workspace,{minTarget:page.viewportSize()!.width<640?40:0});
 const received=page.waitForEvent('download');await workspace.getByRole('link',{name:'原始导入包与资源映射（诊断）',exact:true}).click();
 const download=await received;expect(JSON.parse(await readFile((await download.path())!,'utf8'))).toEqual(pack);
});
test('advanced revision condition preserves applied query and cursor until explicit search',{tag:'@data-closeout'},async({page})=>{
 const reads:URL[]=[];
 await page.route(/\/api\/data\/revisions(?:\?|$)/,route=>{const url=new URL(route.request().url());reads.push(url);return route.fulfill({json:{items:[{kind:'memory',id:'known-object',title:'看雨的记忆',href:'/c/'+fid+'/growth/memory/known-object',history_count:0}],next_cursor:url.searchParams.get('cursor')?'':'second-page'}});});
 await page.goto(origin+'/#/data/revisions?objectId=known-object');
 const workspace=page.getByTestId('domain-workspace');await expect(workspace.getByText('看雨的记忆',{exact:true})).toBeVisible();
 await expect(page.getByRole('dialog')).toHaveCount(0);await expect(workspace.getByText('当前结果已按指定对象筛选。',{exact:true})).toBeVisible();
 await workspace.getByRole('textbox',{name:'标题搜索',exact:true}).fill('未提交搜索');
 await expect(workspace.getByText('筛选输入已改变，点击“查询”后应用。',{exact:true})).toBeVisible();
 await workspace.getByRole('button',{name:'下一页真实对象',exact:true}).click();await expect.poll(()=>reads.length).toBe(2);
 expect(reads[1].searchParams.get('objectId')).toBe('known-object');expect(reads[1].searchParams.get('q')).toBeNull();expect(reads[1].searchParams.get('cursor')).toBe('second-page');
 await workspace.getByRole('button',{name:'精确对象筛选',exact:true}).click();
 const filter=page.getByRole('dialog',{name:'精确对象筛选',exact:true});await filter.getByRole('textbox',{name:'精确对象 ID',exact:true}).fill('another-object');await filter.getByRole('button',{name:'完成',exact:true}).click();
 await expect(filter).toBeHidden();await workspace.getByRole('button',{name:'查询',exact:true}).click();await expect.poll(()=>reads.length).toBe(3);
 expect(reads[2].searchParams.get('objectId')).toBe('another-object');expect(reads[2].searchParams.get('q')).toBe('未提交搜索');expect(reads[2].searchParams.get('cursor')).toBe('');
 await expectNoLayoutDefects(page,workspace,{minTarget:page.viewportSize()!.width<640?40:0});
});

test('account logout keeps the workspace until acknowledgement and uses the shared busy guard',{tag:'@data-closeout'},async({page})=>{
 let release:()=>void=()=>{};const pending=new Promise<void>(resolve=>{release=resolve;});let calls=0,signedOut=false;
 await page.route(/\/auth\/session$/,route=>signedOut?route.fulfill({json:{authenticated:false}}):route.continue());
 await page.route(/\/auth\/logout$/,async route=>{calls++;if(calls===1){await pending;await route.fulfill({status:503,json:{code:'unavailable',message:'退出暂时未能确认，请重试。'}});}else{signedOut=true;await route.fulfill({json:{authenticated:false}});}});
 await page.goto(origin+'/#/library');
 await expect(page.getByRole('link',{name:'去创作中心寻找灵感',exact:true})).toBeVisible();
 const phone=page.viewportSize()!.width<640;
 if(phone){await page.getByRole('button',{name:'打开全部功能',exact:true}).first().click();await page.getByRole('dialog',{name:'探索你的世界'}).getByRole('button',{name:'退出登录',exact:true}).click();}
 else{if(page.viewportSize()!.width>=1024)await page.getByRole('button',{name:'收起导航',exact:true}).click();await expectNoLayoutDefects(page,page.locator('.replica-nav-footer'),{minTarget:40});await page.getByRole('button',{name:'账户菜单',exact:true}).click();await page.getByRole('menuitem',{name:'退出登录',exact:true}).click();}
 await expect.poll(()=>calls).toBe(1);
 if(phone)await expect(page.getByRole('button',{name:'正在退出…',exact:true})).toBeDisabled();
 else{await page.getByRole('button',{name:'账户菜单',exact:true}).click();await expect(page.getByRole('menuitem',{name:'正在退出…',exact:true})).toBeDisabled();}
 expect(page.url()).not.toContain('/auth');release();
 const retry=phone?page.getByRole('button',{name:'退出登录',exact:true}):page.getByRole('menuitem',{name:'退出登录',exact:true});
 await expect(retry).toBeEnabled();expect(page.url()).not.toContain('/auth');await retry.click();
 await expect(page).toHaveURL(/#\/auth$/);expect(calls).toBe(2);
});

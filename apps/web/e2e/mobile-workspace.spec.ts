import {findLayoutDefects,expectNoLayoutDefects} from './support/layout';
import {test,expect} from '@playwright/test';
const origin='http://127.0.0.1:5197';
const fid='fluctlight-fixture-one';
const FIXED_NOW=new Date('2026-11-26T10:30:00+08:00');
test.beforeEach(async({page})=>{
  await page.clock.setFixedTime(FIXED_NOW);
  // Every business response is from the local fixture; deny external traffic.
  await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
});
test('mobile workspace: five destinations, more and full-height conversation', {tag:'@mobile'}, async({page})=>{
  await page.goto(origin+'/#/c/'+fid+'/chat');
  await expect(page.getByRole('textbox',{name:'聊天消息',exact:true})).toBeEnabled();
  if(page.viewportSize()!.width>=640){
    await expect(page.getByRole('navigation',{name:'手机主导航'})).toBeHidden();
    return;
  }
  const nav=page.getByRole('navigation',{name:'手机主导航'});
  await expect(nav).toBeVisible();
  await expect(nav.getByRole('button')).toHaveCount(5);
  await expect(page.locator('.replica-chat > .replica-page-heading')).toBeHidden();
  const outer=await page.locator('.replica-chat-main').evaluate(e=>({h:e.clientHeight,scroll:e.scrollHeight}));
  expect(outer.scroll).toBeLessThanOrEqual(outer.h+1);
  await page.getByRole('button',{name:'会话菜单',exact:true}).click();
  const menu=page.getByRole('dialog',{name:'会话菜单'});
  await expect(menu).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(menu).toBeHidden();
  await nav.getByRole('button',{name:'打开全部功能'}).click();
  const more=page.getByRole('dialog',{name:'探索你的世界'});
  await expect(more).toBeVisible();
  await more.getByRole('button',{name:'日记',exact:true}).click();
  await expect(more).toBeHidden();
  await expect(page).toHaveURL(/life\/diary/);
  await nav.getByRole('button',{name:'角色首页'}).click();
  await expect(page.locator('.mobile-library-welcome')).toBeVisible();
  await page.screenshot({path:test.info().outputPath('mobile-library.png')});
});
test('mobile pages: production routes render using only mock data', {tag:'@mobile'}, async({page})=>{
  test.setTimeout(90_000);
  const routes: [string,string][]=[
    ['/c/'+fid+'/chat','.replica-chat'],['/c/'+fid+'/life/now','.replica-now'],['/c/'+fid+'/life/schedule','.replica-schedule'],['/c/'+fid+'/life/goals','.replica-goals'],['/c/'+fid+'/life/wardrobe','.replica-wardrobe'],['/c/'+fid+'/life/diary','.replica-diary'],['/c/'+fid+'/growth','.replica-memory'],['/c/'+fid+'/profile','.replica-profile'],['/c/'+fid+'/world','.replica-world'],['/c/'+fid+'/people','.replica-people'],['/media','.replica-media'],['/feed','.replica-feed'],['/studio','.replica-studio'],['/settings','.replica-management'],['/library/create','.replica-creation'],['/inbox','.replica-inbox'],['/c/'+fid+'/media/generate','.r-generation-layout'],['/studio/edit/studio-world','[data-testid=studio-editor]'],
  ];
  const defects:unknown[]=[];
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  for(const [route,selector] of routes){
    await page.goto(origin+'/#'+route);
    await expect(page.locator(selector).first()).toBeVisible();
    await page.waitForTimeout(1000);
    defects.push(...(await findLayoutDefects(page,page.locator(".replica-page"),{minTarget:page.viewportSize()!.width<640?40:0})).map(defect=>({route,...defect})));
    await page.screenshot({path:test.info().outputPath(route.replaceAll('/','-')+'.png')});
  }
  expect(errors).toEqual([]);
  expect(defects).toEqual([]);
});

test('phone reading and filters preserve navigation context', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.goto(origin+'/#/c/'+fid+'/life/diary');
  const row=page.locator('.lp-diary-index button').first();
  await expect(row).toBeVisible();await row.click();
  const reader=page.getByRole('dialog');await expect(reader).toBeVisible();
  await expect(reader.getByRole('article',{name:'日记正文'})).toBeVisible();
  await expectNoLayoutDefects(page,reader,{minTarget:40});
  await page.screenshot({path:test.info().outputPath('diary-reader.png')});
  await reader.getByRole('button',{name:'返回日记目录'}).click();
  await expect(reader).toBeHidden();await expect(row).toBeFocused();
  await page.goto(origin+'/#/c/'+fid+'/growth');
  await page.getByRole('button',{name:'筛选',exact:true}).click();
  const filters=page.getByRole('dialog',{name:'筛选记忆'});await expect(filters).toBeVisible();
  await filters.getByRole('combobox',{name:'记忆类型'}).click();
  await page.getByRole('option',{name:'经历',exact:true}).click();
  await filters.getByRole('button',{name:'完成',exact:true}).click();
  await expect(filters).toBeHidden();
  await page.getByRole('button',{name:'筛选',exact:true}).click();
  await expect(page.getByRole('dialog').getByRole('combobox',{name:'记忆类型'})).toContainText('经历');
});
test('media filters: desktop row and phone sheet keep their layout and values', {tag:'@mobile'}, async({page})=>{
  await page.goto(origin+'/#/c/'+fid+'/media');
  const toolbar=page.locator('form.media-toolbar');
  await expect(toolbar.getByRole('searchbox',{name:'搜索喜欢的画面'})).toBeVisible();
  if(page.viewportSize()!.width<640){
    await toolbar.getByRole('button',{name:'筛选',exact:true}).click();
    const sheet=page.getByRole('dialog',{name:'筛选媒体'});
    await expect(sheet).toBeVisible();
    await sheet.getByRole('combobox',{name:'媒体类型'}).click();
    await page.getByRole('option',{name:'视频',exact:true}).click();
    await sheet.getByRole('button',{name:'完成',exact:true}).click();
    await expect(sheet).toBeHidden();
    await toolbar.getByRole('button',{name:'筛选',exact:true}).click();
    await expect(sheet.getByRole('combobox',{name:'媒体类型'})).toContainText('视频');
    await expectNoLayoutDefects(page,sheet,{minTarget:40});
  }else if(page.viewportSize()!.width>=1024){
    for(const width of [1280,1440]){
      await page.setViewportSize({width,height:width===1280?800:900});
      const controls=[toolbar.locator('.r-search'),toolbar.getByRole('combobox',{name:'媒体类型'}),toolbar.getByText('保存的素材 · 画面不改变生活状态',{exact:true}),toolbar.locator('.media-options')];
      const boxes=await Promise.all(controls.map(control=>control.boundingBox()));
      expect(boxes.every(Boolean)).toBeTruthy();
      const centers=boxes.map(box=>box!.y+box!.height/2);
      expect(Math.max(...centers)-Math.min(...centers)).toBeLessThanOrEqual(3);
      await expectNoLayoutDefects(page,toolbar,{minTarget:0});
    }
  }
});
test('settings introduction appears once at every viewport', {tag:'@mobile'}, async({page})=>{
  await page.goto(origin+'/#/settings');
  const management=page.locator('.replica-management');
  await expect(management.getByText('让每一次相处，按你喜欢的方式发生。',{exact:true})).toBeVisible();
  await expect(management.getByText('让相处，按你喜欢的方式发生。',{exact:true})).toHaveCount(0);
  await expect(management.getByRole('heading',{name:'管理中心',exact:true})).toBeVisible();
});
test('phone settings: category directory, save and return', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.goto(origin+'/#/settings');
  await page.getByRole('button',{name:'聊天与视觉',exact:true}).click();
  await expect(page.getByRole('button',{name:'全部设置',exact:true})).toBeVisible();
  await expect(page.getByTestId('config-save-bar')).toBeVisible();
  const timeout=page.getByRole('spinbutton',{name:'总超时（秒，含排队）',exact:true});
  await timeout.fill('126');await page.getByTestId('config-save').click();
  await expect(page.getByTestId('config-save-status')).toContainText(/保存/);
  await expectNoLayoutDefects(page,page.locator('.replica-page'),{minTarget:40});
  await page.getByRole('button',{name:'全部设置',exact:true}).click();
  await expect(page.locator('.mobile-settings-directory')).toBeVisible();
  await page.getByRole('button',{name:'聊天与视觉',exact:true}).click();
  await expect(timeout).toHaveValue('126');
});

test('phone wardrobe: choose, name, save; current wear stays separate', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.goto(origin+'/#/c/'+fid+'/life/wardrobe?tab=compose');
  const wearing=page.locator('.r-wearing-strip');await expect(wearing).toBeVisible();
  const before=await wearing.textContent();
  await page.getByRole('combobox',{name:'挑选衣物的场景'}).click();
  await page.getByRole('option').first().click();
  await page.locator('.r-compose-item.choice').first().click();
  expect(await page.locator('.r-compose-item[aria-pressed=true]').count()).toBeGreaterThan(0);
  await page.getByRole('button',{name:/下一步 · 已选 \d+ 件/}).click();
  const summary=page.locator('.r-compose-summary');await expect(summary).toBeVisible();
  await expect(page.locator('.r-compose-layout > .r-padded')).toBeHidden();
  await summary.getByRole('textbox',{name:'方案名称',exact:true}).fill('手机回归搭配');
  await summary.getByRole('button',{name:'保存为穿搭方案'}).click();
  await expect(page.getByRole('tab',{name:/穿搭方案/})).toHaveAttribute('data-state','active');
  await expect(page.getByRole('heading',{name:'手机回归搭配',exact:true})).toBeVisible();
  await expect(wearing).toHaveText(before || '');
});
test('phone creation: shared world, blank preview, summary and back', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.goto(origin+'/#/library/create');
  await expect(page.getByRole('navigation',{name:'手机主导航'})).toBeHidden();
  await page.getByRole('radio').first().click();
  await page.getByTestId('wizard-world-next').click();
  await expect(page.locator('.creation-mobile-intro h1')).toBeFocused();
  await page.getByRole('button',{name:/从空白开始/}).click();
  await page.getByTestId('wizard-name').fill('手机测试相识');
  await page.getByTestId('wizard-analyze').click();
  await expect(page.locator('.creation-blank-preview')).toContainText('手机测试相识');
  await page.getByRole('button',{name:'摘要',exact:true}).click();
  const summary=page.getByRole('dialog',{name:'这次相遇的摘要'});await expect(summary).toContainText('手机测试相识');
  await page.keyboard.press('Escape');
  await page.getByRole('button',{name:'上一步',exact:true}).click();
  await expect(page.getByTestId('wizard-name')).toHaveValue('手机测试相识');
  await page.getByTestId('wizard-analyze').click();
  await page.getByTestId('wizard-activate').click();
  await expect(page.locator('.creation-progress-number')).toHaveText(/7\s*\/\s*7/);
  await expect(page.getByRole('button',{name:/全部准备事项/})).toBeVisible();
  await page.getByRole('button',{name:/全部准备事项/}).click();
  await expect(page.getByRole('list',{name:'准备相遇的实际进度'}).getByRole('listitem')).toHaveCount(7);
  await page.screenshot({path:test.info().outputPath('creation-prepared.png')});
});
test('phone viewport: small screens, navigation breakpoints and keyboard space', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Runs once from phone project');
  test.setTimeout(60_000);
  await page.goto(origin+'/#/c/'+fid+'/chat');
  await expect(page.getByRole('textbox',{name:'聊天消息',exact:true})).toBeEnabled();
  const nav=page.getByRole('navigation',{name:'手机主导航'});
  for(const width of [360,390,430,639,640,768,1023,1024]){
    await page.setViewportSize({width,height:width===360?740:width===430?932:width<640?844:1024});
    if(width<640)await expect(nav).toBeVisible();else await expect(nav).toBeHidden();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  }
  await page.setViewportSize({width:390,height:844});
  const input=page.getByRole('textbox',{name:'聊天消息',exact:true});await input.fill('键盘打开时保留的草稿');
  await page.evaluate(()=>{Object.defineProperty(window.visualViewport!,'height',{configurable:true,value:584});window.visualViewport!.dispatchEvent(new Event('resize'));});
  await expect(nav).toBeHidden();await expect(input).toHaveValue('键盘打开时保留的草稿');
  const composer=await page.locator('.companion-composer').boundingBox();expect(composer!.y+composer!.height).toBeLessThanOrEqual(585);
  await page.evaluate(()=>{delete (window.visualViewport as any).height;window.visualViewport!.dispatchEvent(new Event('resize'));});
  await expect(nav).toBeVisible();
});
test('phone failure recovery: failed memory refresh keeps the reader reachable', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  let fail=false;
  await page.route('**/api/fluctlights/*/memories?*',route=>fail?(fail=false,route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({message:'手机mock读取失败'})})):route.continue());
  await page.goto(origin+'/#/c/'+fid+'/growth');
  await page.getByRole('button',{name:'筛选',exact:true}).click();
  const refresh=page.getByRole('button',{name:'刷新记忆',exact:true});await expect(refresh).toBeEnabled();
  fail=true;await refresh.click();
  await page.getByRole('dialog').getByRole('button',{name:'完成',exact:true}).click();
  await expect(page.getByText('记忆暂未读全',{exact:true})).toBeVisible();
  await expect(page.getByText('上次确认的内容仍保留。',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'筛选',exact:true}).click();await refresh.click();
  await page.getByRole('dialog').getByRole('button',{name:'完成',exact:true}).click();
  await expect(page.getByText('记忆暂未读全',{exact:true})).toBeHidden();
  await expect(page.getByRole('navigation',{name:'手机主导航'})).toBeVisible();
});

test('phone studio: sandbox returns to the requested publishing section', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.goto(origin+'/#/studio/preview?res=studio-world&kind=world');
  await page.getByRole('tab',{name:'发布与使用',exact:true}).click();
  await expect(page).toHaveURL(/section=publish/);
  await expect(page.getByRole('tab',{name:'发布与使用',exact:true})).toHaveAttribute('data-state','active');
  await expect(page.getByRole('heading',{name:'保存与发布',exact:true})).toBeVisible();
});
test('phone account: focused login and inline validation', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.route('**/auth/session',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({authenticated:false})}));
  await page.goto(origin+'/#/auth');
  await expect(page.getByRole('heading',{name:'欢迎回来'})).toBeVisible();
  await expect(page.getByRole('navigation',{name:'手机主导航'})).toBeHidden();
  await expectNoLayoutDefects(page,page.locator('.replica-auth-page'),{minTarget:40});
  await page.getByLabel('所有者密码',{exact:true}).fill('short');
  await page.getByRole('button',{name:/进入|登录|回到/}).click();
  await expect(page.getByRole('alert').filter({hasText:'密码至少需要 12 个字符。'})).toBeVisible();
});
test('phone visual review: compare first, decide second, preserve draft on return', {tag:'@mobile'}, async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.goto(origin+'/#/c/'+fid+'/media/identity');
  await page.getByRole('button',{name:'查看候选并评审',exact:true}).click();
  const review=page.getByRole('dialog');await expect(review).toBeVisible();
  await expect(review.locator('.r-review-reference')).toBeVisible();
  await expect(review.locator('.r-review-decision')).toBeHidden();
  await review.getByRole('tab',{name:'2 填写决定'}).click();
  await review.getByRole('radio',{name:'不通过',exact:true}).click();
  await review.getByRole('textbox',{name:'不通过的原因',exact:true}).fill('侧面信息需要更清晰');
  await review.getByRole('tab',{name:'1 图片对照'}).click();
  await review.getByRole('tab',{name:'2 填写决定'}).click();
  await expect(review.getByRole('textbox',{name:'不通过的原因',exact:true})).toHaveValue('侧面信息需要更清晰');
  await expectNoLayoutDefects(page,review,{minTarget:40});
  await page.keyboard.press('Escape');await expect(review).toBeHidden();
});

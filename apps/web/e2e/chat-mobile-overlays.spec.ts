import {test,expect} from '@playwright/test';
import {expectNoLayoutDefects} from './support/layout';
const origin='http://127.0.0.1:5196'; // Built production assets + isolated HTTP fixture.
const fid='fluctlight-fixture-one';
test.beforeEach(async({page})=>{
  test.skip(page.viewportSize()!.width>=640,'Phone presentation only');
  await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
  await page.goto(origin+'/#/c/'+fid+'/chat');
  await expect(page.getByRole('textbox',{name:'聊天消息',exact:true})).toBeEnabled();
});
for(const size of [{width:360,height:740},{width:390,height:844},{width:430,height:932}]) test(`built mobile chat: more menu and history stay inside ${size.width}x${size.height}`, {tag:['@mobile','@overlay']}, async({page})=>{
  await page.setViewportSize(size);
  await page.route('**/api/conversations/*/messages?*',async route=>{
    const response=await route.fetch();const body=await response.json();
    body.messages=body.messages.map((message:any,index:number)=>index===0?{...message,text:'这一段用于验证长会话历史的滚动与底部操作。'.repeat(80)}:message);
    await route.fulfill({response,json:body});
  });
  await page.getByRole('button',{name:'会话菜单',exact:true}).click();
  const menu=page.getByRole('dialog',{name:'会话菜单',exact:true});
  await expect(menu).toBeVisible();
  await expectNoLayoutDefects(page,menu,{minTarget:40});
  const menuBox=(await menu.boundingBox())!;
  expect(menuBox.x).toBeCloseTo(0,0);expect(menuBox.width).toBeCloseTo(size.width,0);
  expect(menuBox.y+menuBox.height).toBeCloseTo(size.height,0);
  await menu.getByRole('button',{name:'会话历史',exact:true}).click();
  await expect(menu).toBeHidden();
  const history=page.getByRole('dialog',{name:'那些聊过的日常',exact:true});
  await expect(history).toBeVisible();
  await expectNoLayoutDefects(page,history,{minTarget:40});
  const box=(await history.boundingBox())!;
  expect(box.x).toBeCloseTo(0,0);expect(box.y).toBeCloseTo(0,0);
  expect(box.width).toBeCloseTo(size.width,0);expect(box.height).toBeCloseTo(size.height,0);
  await expect(history.getByRole('heading',{name:'那些聊过的日常'})).toBeInViewport();
  await expect(history.getByRole('button',{name:'关闭',exact:true})).toBeInViewport();
  const body=history.locator('.replica-dialog-body');
  await expect.poll(()=>body.evaluate(el=>el.scrollHeight>el.clientHeight)).toBe(true);
  await body.evaluate(el=>el.scrollTop=el.scrollHeight);
  await expect(history.getByRole('button',{name:'继续这段对话',exact:true})).toBeInViewport();
  await expect(history.getByRole('heading',{name:'那些聊过的日常'})).toBeInViewport();
  await page.screenshot({path:test.info().outputPath('built-chat-history.png')});
  await page.keyboard.press('Escape');
  const trigger=page.getByRole('button',{name:'会话历史',exact:true});
  await expect(trigger).toBeFocused();await trigger.click();
  await expect(history).toBeVisible();await expectNoLayoutDefects(page,history,{minTarget:40});
});
test('built mobile chat: composer has no plus shortcut and retains the draft', {tag:'@mobile'}, async({page})=>{
  await expect(page.getByRole('button',{name:'聊天工具',exact:true})).toHaveCount(0);
  await expect(page.locator('.mobile-composer-tool')).toHaveCount(0);
  const input=page.getByRole('textbox',{name:'聊天消息',exact:true});
  await input.fill('保留这条聊天草稿');
  await page.getByRole('button',{name:'会话菜单',exact:true}).click();
  await page.keyboard.press('Escape');await expect(input).toHaveValue('保留这条聊天草稿');
  await expect(page.getByRole('button',{name:'发送消息',exact:true})).toBeEnabled();
});

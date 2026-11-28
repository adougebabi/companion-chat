import {test,expect} from '@playwright/test';
import {expectNoLayoutDefects,expectTabsReachable} from './support/layout';
const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
const pages=[['日程','life/schedule','.replica-schedule'],['目标','life/goals','.replica-goals'],['日记','life/diary','.replica-diary'],['人物','people','.replica-people'],['世界','world','.replica-world'],['衣柜','life/wardrobe','.replica-wardrobe'],['资料','profile','.replica-profile']];
test.beforeEach(async({page})=>{await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());});
for(const [name,path,selector] of pages)test('shared responsive tabs preserve the controlled section: '+name,{tag:'@shared-tabs'},async({page})=>{
 await page.goto(origin+'/#/c/'+fid+'/'+path);
 const root=page.locator(selector).first();
 if(name==='世界')await root.getByRole('button',{name:/^进入世界 /}).first().click();
 const list=root.locator('[data-responsive-tabs]').first();
 await expect(list).toBeVisible({timeout:20000});await expectTabsReachable(list);
 const tabs=list.getByRole('tab');
 for(let index=0;index<await tabs.count();index++){
  const tab=tabs.nth(index);if(await tab.isDisabled())continue;
  await tab.scrollIntoViewIfNeeded();await tab.click();await expect(tab).toHaveAttribute('aria-selected','true');
  await expectNoLayoutDefects(page,list,{minTarget:page.viewportSize()!.width<640?40:0});
 }
 if(page.viewportSize()!.width<640){for(const width of [360,430]){await page.setViewportSize({width,height:844});await expectTabsReachable(list);await expectNoLayoutDefects(page,list,{minTarget:40});}}
});
test('profile governance and person record tabs use the same shared navigation',{tag:'@shared-tabs'},async({page})=>{
 await page.goto(origin+'/#/c/'+fid+'/profile');await page.getByRole('button',{name:'身份与能力',exact:true}).click();
 const governance=page.getByRole('dialog',{name:'身份与能力',exact:true}),list=governance.locator('[data-responsive-tabs]').first();
 await expect(list).toBeVisible();await expectTabsReachable(list);
 for(const name of ['身份核心提议','能力申请','发展中的自我']){const tab=list.getByRole('tab',{name,exact:true});await tab.click();await expect(tab).toHaveAttribute('aria-selected','true');}
 await expectNoLayoutDefects(page,list,{minTarget:page.viewportSize()!.width<640?40:0});await page.keyboard.press('Escape');
 await page.goto(origin+'/#/c/'+fid+'/people');await page.getByRole('button',{name:/^查看人物 /}).first().click();
 const detail=page.getByRole('dialog').last();await detail.getByRole('button',{name:'接触、在场与相遇记录',exact:true}).click();
 const records=detail.locator('[data-responsive-tabs]').first();await expect(records).toBeVisible();await expectTabsReachable(records);
 for(let i=0;i<await records.getByRole('tab').count();i++){const tab=records.getByRole('tab').nth(i);await tab.click();await expect(tab).toHaveAttribute('aria-selected','true');}
 await expectNoLayoutDefects(page,records,{minTarget:page.viewportSize()!.width<640?40:0});
});

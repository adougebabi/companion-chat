import {test,expect} from '@playwright/test';
import {readManifest,verifyBaselines} from './support/baselines.mjs';
import {expectNoLayoutDefects} from './support/layout';
const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
test.beforeEach(async({page})=>{
 await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
});
test('goal object URL uses the shared detail and editor even when the goal is outside the loaded list',{tag:['@goals','@visual']},async({page})=>{
 await page.route(/\/api\/fluctlights\/[^/]+\/goals\?/,route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({items:[],next_cursor:''})}));
 await page.goto(origin+'/#/c/'+fid+'/life/goal/goal-fixture-one');
 const detail=page.getByRole('dialog',{name:'完成一幅新的水彩作品'});
 await expect(detail).toBeVisible({timeout:20000});
 await expect(detail).toContainText('完成并保存画作');
 await expect(detail.getByRole('link',{name:'目标列表'})).toHaveAttribute('href','#/c/'+fid+'/life/goals');
 await expect(detail.getByRole('link',{name:'独立规划'})).toHaveAttribute('href','#/c/'+fid+'/life/planning');
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 const name='goal-details-object.png',file=test.info().project.name+'/'+name;
 if(!process.env.UI_BASELINE_APPROVAL_REASON&&(verifyBaselines().status!=='ok'||!readManifest()?.files?.[file]))throw Error('UI_BASELINE_UNVERIFIED: '+file);
 await expect(detail).toHaveScreenshot(name);
 if(page.viewportSize()!.width<640){await page.setViewportSize({width:360,height:740});await expect.poll(async()=>Math.round((await detail.boundingBox())!.height)).toBe(740);await expectNoLayoutDefects(page,detail,{minTarget:40});}
 await detail.getByRole('button',{name:'编辑',exact:true}).click();
 const editor=page.getByRole('dialog',{name:'编辑目标'});
 await expect(editor).toBeVisible();
 await expect(editor.getByRole('combobox',{name:'目标范围'})).toBeDisabled();
 await editor.getByRole('textbox',{name:'目标内容'}).fill('尚未提交的目标');
 await editor.getByRole('button',{name:'取消',exact:true}).click();
 await expect(detail).toBeVisible();
 await expect(detail).not.toContainText('尚未提交的目标');
 await page.keyboard.press('Escape');
 await expect(page).toHaveURL(new RegExp('/c/'+fid+'/life/goals$'));
 await expect(page.getByRole('dialog')).toHaveCount(0);
});
test('goal list still opens the same definition, records and command form',{tag:'@goals'},async({page})=>{
 await page.goto(origin+'/#/c/'+fid+'/life/goals');
 await page.getByRole('button',{name:'查看目标 完成一幅新的水彩作品',exact:true}).click();
 const detail=page.getByRole('dialog',{name:'完成一幅新的水彩作品'});
 await expect(detail).toContainText('完成并保存画作');
 await expect(detail).toContainText('尚无证据来源。');
 await detail.getByRole('button',{name:'目标修订历史',exact:true}).click();
 await expect(detail.locator('.replica-goal-record')).toContainText('验收样本');
 await expect(detail.getByRole('button',{name:'原始目标修订记录（诊断）',exact:true})).toBeVisible();
 await expect(detail.getByRole('button',{name:'原始目标与推进记录（诊断）'})).toBeVisible();
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 await page.keyboard.press('Escape');
 await expect(page.getByRole('button',{name:'查看目标 完成一幅新的水彩作品',exact:true})).toBeFocused();
});

test('unread goal failure stays readonly, retry keeps its ID and long content remains readable',{tag:'@goals'},async({page})=>{
 await page.route(/\/api\/fluctlights\/[^/]+\/goals\?/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
 let reads=0;
 const longTitle='整理这周的街景画册 '+('long-source-without-spaces-'.repeat(8));
 await page.route(/\/api\/fluctlights\/[^/]+\/goals\/goal-fixture-one$/,async route=>{
  if(reads++===0){await new Promise(resolve=>setTimeout(resolve,350));await route.fulfill({status:503,json:{code:'service_unavailable',message:'目标资料暂时无法读取。'}});return;}
  const response=await route.fetch(),goal=await response.json();
  await route.fulfill({json:{...goal,desired_outcome:longTitle,motivation:'把这周观察到的街景与对话整理成图册，保留实际完成依据。'.repeat(8),success_criteria:['完成并保存三张画作','核对实际使用的素材来源','保留修订和评估记录'],criterion_ids:['criterion-long-1','criterion-long-2','criterion-long-3']}});
 });
 await page.goto(origin+'/#/c/'+fid+'/life/goal/goal-fixture-one');
 const detail=page.getByRole('dialog');
 await expect(detail.getByText('正在读取目标详情…',{exact:true})).toBeVisible();
 await expect(detail.getByRole('button',{name:'重新读取目标详情'})).toBeVisible();
 await expect(detail.getByRole('button',{name:'编辑',exact:true})).toHaveCount(0);
 await expect(detail.getByRole('button',{name:'原始目标读取错误记录（诊断）',exact:true})).toBeVisible();
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 await detail.getByRole('button',{name:'重新读取目标详情'}).click();
 await expect(detail.getByRole('heading',{name:longTitle,exact:true})).toBeVisible();
 await expect(detail.getByRole('button',{name:'原始目标读取错误记录（诊断）',exact:true})).toHaveCount(0);
 expect(await detail.getByRole('heading',{name:longTitle,exact:true}).evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 await page.screenshot({path:test.info().outputPath('goal-long-after-retry.png')});
 expect(reads).toBe(2);
});

test('cached goal content stays readable after detail failure but only a confirmed detail enables writes',{tag:'@goals'},async({page})=>{
 let reads=0;
 await page.route(/\/api\/fluctlights\/[^/]+\/goals\/goal-fixture-one$/,async route=>{
  if(reads++===0){await new Promise(resolve=>setTimeout(resolve,600));await route.fulfill({status:503,json:{code:'unavailable',message:'目标详情需要重新读取。'}});return;}
  await route.continue();
 });
 await page.goto(origin+'/#/c/'+fid+'/life/goal/goal-fixture-one');
 const detail=page.getByRole('dialog',{name:'完成一幅新的水彩作品'});
 await expect(detail.getByRole('button',{name:'重新读取目标详情',exact:true})).toBeVisible();
 await expect(detail.locator('.lp-check-line').filter({hasText:'完成并保存画作'})).toBeVisible();
 for(const name of ['编辑','暂停','核对进展','放下这个目标','取消这个目标'])await expect(detail.getByRole('button',{name,exact:true})).toBeDisabled();
 await detail.getByRole('button',{name:'重新读取目标详情',exact:true}).click();
 for(const name of ['编辑','暂停','放下这个目标','取消这个目标'])await expect(detail.getByRole('button',{name,exact:true})).toBeEnabled();
 // The real fixture still has a pending evaluation; confirming the detail must not bypass it.
 await expect(detail.getByRole('button',{name:'核对进展',exact:true})).toBeDisabled();
 expect(reads).toBe(2);
});

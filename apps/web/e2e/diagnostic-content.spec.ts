import {test,expect} from '@playwright/test';
import {expectNoLayoutDefects} from './support/layout';
import {readManifest,verifyBaselines} from './support/baselines.mjs';
const origin='http://127.0.0.1:'+Number(process.env.UI_SCENARIO_PORT||5198);
async function settle(page:any){
 await page.evaluate(()=>document.fonts.ready);
 await expect.poll(()=>page.evaluate(()=>document.getAnimations().filter(a=>a.playState==='running'&&a.effect?.getTiming().iterations!==Infinity).length)).toBe(0);
}
function approved(name:string){
 if(process.env.UI_BASELINE_APPROVAL_REASON)return;
 const store=verifyBaselines(),file=test.info().project.name+'/'+name;
 if(store.status!=='ok'||!readManifest()?.files?.[file])throw Error('UI_BASELINE_UNVERIFIED: '+file);
}
test.describe('diagnostic business content',{tag:['@task-diagnostics','@visual']},()=>{
 test('call content, immediate technical navigation, read-only replay and original record',async({page})=>{
  await page.clock.setFixedTime(new Date('2026-11-16T10:30:00+08:00'));
  await page.goto(origin+'/?scenario=calls-long');
  await page.locator('[data-testid^=diag-open-]').first().click();
  const dialog=page.getByRole('dialog').last();
  await expect(dialog.getByRole('tab',{name:'请求与返回'})).toHaveAttribute('data-state','active');
  await settle(page);
  await expectNoLayoutDefects(page,dialog,{minTarget:page.viewportSize()!.width<640?40:0});
  approved('diagnostic-call-content.png');
  await expect(dialog).toHaveScreenshot('diagnostic-call-content.png');
  await dialog.getByRole('button',{name:'技术详情与关联标识',exact:true}).click();
  await expect(dialog.getByRole('tab',{name:'调用信息'})).toHaveAttribute('data-state','active');
  await expect(dialog.locator('[data-ui-detail-fields]')).toBeVisible();
  await dialog.getByRole('tab',{name:'只读回放此运行'}).click();
  await expect(dialog.getByTestId('diagnostic-replay')).toContainText('已读取');
  await expect(dialog.getByTestId('diagnostic-replay')).not.toContainText('读取失败');
  await expect(dialog.getByRole('list',{name:'已读取的运行过程'})).toBeVisible();
  await expect(dialog.getByRole('link',{name:'查看此条完整记录与来源'})).toBeVisible();
  await dialog.getByRole('button',{name:/^完整回放记录/}).click();
  await expect(dialog.getByTestId('replay-record').locator('pre')).toContainText('corr-long');
  await expectNoLayoutDefects(page,dialog,{minTarget:page.viewportSize()!.width<640?40:0});
 });
 test('event list: applied filters stay independent, paging and source remain reachable',async({page})=>{
  await page.clock.setFixedTime(new Date('2026-11-16T10:30:00+08:00'));
  await page.goto(origin+'/?scenario=calls-long#/diag/events');
  const records=page.locator('[data-records]');
  await expect(records.locator('.diagnostic-card')).toHaveCount(3);
  await expect(page.locator('[data-applied]')).toHaveText('已应用：全部记录');
  await expect(records).not.toContainText('本次执行未完成');
  const firstHref=await records.getByRole('link',{name:'完整记录与来源'}).first().getAttribute('href');
  await records.getByRole('button',{name:'下一页系统诊断事件'}).click();
  await expect(records.getByRole('link',{name:'完整记录与来源'}).first()).not.toHaveAttribute('href',firstHref!);
  await records.getByRole('button',{name:'上一页',exact:true}).click();
  await expect(records.getByRole('link',{name:'完整记录与来源'}).first()).toHaveAttribute('href',firstHref!);
  await settle(page);
  await expectNoLayoutDefects(page,page.locator('.diagnostic-page'),{minTarget:page.viewportSize()!.width<640?40:0});
  approved('diagnostic-events.png');
  await expect(page).toHaveScreenshot('diagnostic-events.png');
  await page.getByRole('textbox',{name:'记录内容搜索'}).fill('workflow.failed');
  await page.getByRole('button',{name:'刷新',exact:true}).click();
  await expect(records.locator('.diagnostic-card')).toHaveCount(3);
  await expect(page.locator('[data-applied]')).toHaveText('已应用：全部记录');
  await page.getByRole('button',{name:'查询',exact:true}).click();
  await expect(page.locator('[data-applied]')).toContainText('workflow.failed');
  await expect(records).toContainText('未完成');
  await records.getByRole('link',{name:'完整记录与来源'}).first().click();
  await expect(page).toHaveURL(/diag\/record\/events/);
  await expect(page.getByRole('button',{name:'重新读取实际状态'})).toBeVisible();
 });
 test('advanced filter uses shared sheet on phone and retains draft after closing',async({page})=>{
  await page.goto(origin+'/?scenario=calls-long#/diag/events');
  await page.getByRole('button',{name:/更多筛选条件/}).click();
  const mobile=page.viewportSize()!.width<640;
  const scope=mobile?page.getByRole('dialog',{name:'高级筛选'}):page;
  await scope.getByRole('textbox',{name:'关联 ID',exact:true}).fill('corr-failed');
  if(mobile)await scope.getByRole('button',{name:'关闭',exact:true}).click();
  await page.getByRole('button',{name:'查询',exact:true}).click();
  await expect(page.locator('[data-applied]')).toContainText('corr-failed');
  await expect(page.locator('[data-records]')).toContainText('任务详情');
 });
});

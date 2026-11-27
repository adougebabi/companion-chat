import { expect, test, type Page } from '@playwright/test';
import { expectNoLayoutDefects, expectTabsReachable } from './support/layout';

const FIXED_NOW = new Date('2026-11-16T10:30:00+08:00');
async function open(page: Page, query = 'section=promises') {
  await page.clock.setFixedTime(FIXED_NOW);
  await page.goto(`/ui-lab/?fixture=schedule-settings&${query}`);
  const workspace = page.getByTestId('domain-workspace');
  await expect(workspace.getByRole('heading', { name: '约定与意向', exact: true })).toBeVisible();
  await expect(workspace.getByRole('tablist', { name: '日程分区' })).toBeVisible();
  return workspace;
}
const events = (page: Page) => page.evaluate(() => (window as any).__uiLabEvents as { action: string; args: any[] }[]);
const minTarget = (page: Page) => page.viewportSize()!.width < 640 ? 40 : 0;

test.describe('约定与意向 planning workspace', { tag: ['@schedule'] }, () => {
  test('direct route exposes content and all three sections without opening an overlay', async ({ page }) => {
    const workspace = await open(page);
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(workspace.getByText('周六一起去看展')).toBeVisible();
    await expect(workspace.getByRole('link', { name: '返回当天安排与生活倾向' })).toHaveAttribute('href', '#/c/A/life/schedule');
    const list = workspace.getByRole('tablist');
    await expectTabsReachable(list);
    for (const label of ['约定', '意向', '接下来几天']) {
      await list.getByRole('tab', { name: label, exact: true }).click();
      await expect(list.getByRole('tab', { name: label, exact: true })).toHaveAttribute('aria-selected', 'true');
      if (label === '接下来几天') await expect(workspace.getByRole('heading', { name: '2026-11-17' })).toBeVisible();
      await expectNoLayoutDefects(page, workspace, { minTarget: minTarget(page) });
    }
    expect((await events(page)).some(e => e.action === 'schedules' && e.args[1]?.from === '2026-11-17' && e.args[1]?.to === '2026-11-23')).toBe(true);
  });

  test('date selection stays on the production planning route and selected section', async ({ page }) => {
    const workspace = await open(page, 'section=upcoming');
    await workspace.getByRole('combobox', { name: '从哪天起查看' }).click();
    const calendar = page.locator('[data-slot="popover-content"]');
    await expect(calendar).toBeVisible();
    await expectNoLayoutDefects(page, calendar);
    await calendar.getByRole('button', { name: /18/ }).first().click();
    await workspace.getByRole('button', { name: '查找' }).click();
    await expect(workspace.getByRole('heading', { name: '2026-11-19' })).toBeVisible();
    expect((await events(page)).some(e => e.action === 'navigate' && e.args[0].includes('/life/schedule/planning?date=2026-11-18&section=upcoming'))).toBe(true);
    await expect(workspace.getByRole('tab', { name: '接下来几天' })).toHaveAttribute('aria-selected', 'true');
    await workspace.getByRole('button', { name: '回到今天' }).click();
    await expect(workspace.getByRole('heading', { name: '2026-11-17' })).toBeVisible();
  });

  test('new schedule suggestion opens directly with a reachable fixed submit and returns focus', async ({ page }) => {
    const workspace = await open(page);
    await expect(workspace.getByRole('link', { name: '到聊天中发起邀请' })).toHaveAttribute('href', '#/c/A/chat');
    const trigger = workspace.getByRole('button', { name: '填写日程建议' });
    await trigger.click();
    const form = page.getByRole('dialog', { name: '建议一项日程' });
    await expect(form).toBeVisible();
    await expect.poll(() => form.evaluate(el => el.getAnimations().filter(a => a.playState === 'running').length)).toBe(0);
    await expectNoLayoutDefects(page, form, { minTarget: minTarget(page) });
    await expect(form.getByRole('button', { name: '发出建议' })).toBeInViewport({ ratio: 1 });
    await form.locator(':scope > [data-slot="sheet-close"]').click();
    await expect(form).toBeHidden();
    await expect(trigger).toBeFocused();
  });

  test('promise detail and proposal preserve the real controller actions', async ({ page }) => {
    const workspace = await open(page);
    const trigger = workspace.getByRole('button', { name: '查看约定' }).first();
    await trigger.click();
    const detail = page.getByRole('dialog', { name: '约定与履行证据' });
    await expect(detail.getByRole('heading', { name: '周六一起去看展' })).toBeVisible();
    await expectNoLayoutDefects(page, detail, { minTarget: minTarget(page) });
    expect((await events(page)).some(e => e.action === 'readPromise' && e.args[0] === 'promise-1')).toBe(true);
    await detail.locator('footer').getByRole('button', { name: '取消', exact: true }).click();
    await expect(detail).toBeHidden();
    await expect(trigger).toBeFocused();
    await workspace.getByRole('button', { name: '和她约定' }).click();
    const propose = page.getByRole('dialog');
    await expect(propose).toBeVisible();
    await expectNoLayoutDefects(page, propose, { minTarget: minTarget(page) });
  });

  test('long future arrangements remain readable in the page scroll owner', async ({ page }) => {
    const workspace = await open(page, 'section=upcoming&variant=long');
    await workspace.getByRole('button', { name: /在旧街尽头/ }).first().click();
    await expectNoLayoutDefects(page, workspace, { minTarget: minTarget(page) });
    await workspace.getByRole('heading', { name: '2026-11-23' }).scrollIntoViewIfNeeded();
    await expect(workspace.getByRole('heading', { name: '2026-11-23' })).toBeInViewport();
    await expect(page.getByRole('dialog')).toHaveCount(0);
  });

  test('empty promises and future days show independent empty states', async ({ page }) => {
    const workspace = await open(page, 'section=promises&variant=empty');
    await expect(workspace.getByText(/还没有约定提议/)).toBeVisible();
    await workspace.getByRole('tab', { name: '接下来几天' }).click();
    await expect(workspace.getByText('这一天尚无已接受日程。').first()).toBeVisible();
    await expectNoLayoutDefects(page, workspace, { minTarget: minTarget(page) });
  });
});

test('main schedule source link opens the same flat planning route with real workspace navigation', {tag:'@schedule'}, async({page})=>{
  const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
  await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
  await page.route(/\/api\/fluctlights\/[^/]+\/promises(?:\?|$)/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
  await page.goto(origin+'/#/c/'+fid+'/life/schedule');
  await page.getByRole('link',{name:'查看约定与意向',exact:true}).click();
  const workspace=page.getByTestId('domain-workspace');
  await expect(workspace.getByRole('heading',{name:'约定与意向',exact:true})).toBeVisible();
  await expect(workspace.getByRole('tab',{name:'约定',exact:true})).toHaveAttribute('aria-selected','true');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(workspace.getByText(/还没有约定提议/)).toBeVisible();
  await expectNoLayoutDefects(page,workspace,{minTarget:minTarget(page)});
  await workspace.getByRole('link',{name:'返回当天安排与生活倾向'}).click();
  await expect(page).toHaveURL(new RegExp('/c/'+fid+'/life/schedule$'));
  await expect(page.getByRole('link',{name:'查看约定与意向',exact:true})).toBeVisible();
});

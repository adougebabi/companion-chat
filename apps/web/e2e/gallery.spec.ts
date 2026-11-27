import { expect, test, type Page } from '@playwright/test';
import { GALLERY_CATALOG } from '../ui-lab/fixtures/gallery-catalog';
import { expectNoLayoutDefects } from './support/layout';

// Shared-pattern interactions and the gallery index in real Chromium. The
// per-state layout checks run in visual.spec.ts, in the same page load as the
// approved screenshot. Fixture data is fixed; the clock is frozen.
const FIXED_NOW = new Date('2026-11-16T10:30:00+08:00');
const isPhone = (page: Page) => page.viewportSize()!.width < 640;
const events = (page: Page) => page.evaluate(() => (window as any).__uiLabEvents as { action: string; args: any[] }[]);

async function open(page: Page, component: string, state: string) {
  await page.clock.setFixedTime(FIXED_NOW);
  await page.goto(`/ui-lab/?fixture=gallery&component=${component}&state=${state}`);
  await expect(page.locator(`[data-gallery-target][data-state="${state}"]`)).toBeAttached();
}

test('gallery index lists every component and links to its first state', { tag: '@patterns' }, async ({ page }) => {
  await page.goto('/ui-lab/?fixture=gallery');
  const list = page.getByRole('list', { name: '组件列表' });
  await expect(list.getByRole('listitem')).toHaveCount(Object.keys(GALLERY_CATALOG).length);
  await expectNoLayoutDefects(page, page.locator('[data-ui-lab-page]'), { minTarget: isPhone(page) ? 40 : 0 });
});

test.describe('interactions', { tag: ['@patterns', '@overlay'] }, () => {
  test('new settings form built from shared patterns submits native form data', async ({ page }) => {
    await open(page, 'settings-form', 'normal');
    const sheet = page.getByRole('dialog', { name: '通知偏好设置' });
    await sheet.getByRole('switch', { name: '新的动态发布时提醒我' }).click();
    await sheet.getByLabel('开始').fill('22:30');
    await sheet.getByRole('combobox', { name: '摘要频率' }).click();
    await page.getByRole('option', { name: '每周一次' }).click();
    await sheet.getByLabel('备注').fill('周末可以晚一点提醒');
    await sheet.getByRole('button', { name: '保存设置' }).click();
    await expect(sheet).toBeHidden();
    const submitted = (await events(page)).find(e => e.action === 'settings-submit')!.args[0];
    expect(submitted).toMatchObject({ proactive: 'on', moment: 'on', promise: 'on', quietStart: '22:30', quietEnd: '07:30', digest: 'weekly', note: '周末可以晚一点提醒' });
  });

  test('edit sheet: required field blocks submit; busy sheet cannot be dismissed', async ({ page }) => {
    await open(page, 'edit-sheet', 'normal');
    const sheet = page.getByRole('dialog', { name: '编辑约定' });
    await sheet.getByRole('textbox', { name: '约定内容' }).fill('');
    await sheet.getByRole('button', { name: '保存' }).click();
    await expect(sheet).toBeVisible();
    expect((await events(page)).some(e => e.action === 'edit-submit')).toBe(false);
    await sheet.getByRole('textbox', { name: '约定内容' }).fill('周日去公园');
    await sheet.getByRole('button', { name: '保存' }).click();
    await expect(sheet).toBeHidden();
    expect((await events(page)).find(e => e.action === 'edit-submit')!.args[0]).toMatchObject({ title: '周日去公园', start: '10:00', end: '18:00' });

    await open(page, 'edit-sheet', 'busy');
    const busy = page.getByRole('dialog', { name: '编辑约定' });
    await expect(busy.getByRole('button', { name: '正在保存…' })).toBeDisabled();
    await expect(busy.getByRole('button', { name: '关闭' })).toHaveCount(0);
    await page.keyboard.press('Escape');
    await expect(busy).toBeVisible();
  });

  test('edit sheet error is announced and focused', async ({ page }) => {
    await open(page, 'edit-sheet', 'error');
    const alert = page.getByRole('dialog').getByRole('alert').first();
    await expect(alert).toContainText('草稿已保留');
    await expect(page.locator(':focus')).toContainText('草稿已保留');
  });

  test('tabs: keyboard navigation, disabled tab skipped, active tab visible', async ({ page }) => {
    await open(page, 'tabs', 'disabled');
    const list = page.getByRole('tablist', { name: '日程分区' });
    await list.getByRole('tab', { name: '当天安排' }).click();
    // Radix moves roving focus on a timer: wait for each move before the next key.
    await page.keyboard.press('ArrowRight');
    await expect(list.getByRole('tab', { name: '接下来几天' })).toBeFocused();
    await page.keyboard.press('ArrowRight');
    await expect(list.getByRole('tab', { name: '约定' })).toBeFocused();
    await expect(list.getByRole('tab', { name: '意向' })).toBeDisabled();
    await open(page, 'tabs', 'long');
    const active = page.getByRole('tab', { selected: true });
    await expect(active).toHaveText('历史记录与原始请求');
    await expect(active).toBeInViewport({ ratio: 1 });
  });

  test('migrated DetailDrawer keeps its test ids, footer and close behaviour', async ({ page }) => {
    await open(page, 'migrated-detail-drawer', 'normal');
    const sheet = page.getByTestId('migrated-drawer');
    await expect(sheet).toHaveAttribute('data-responsive-sheet', '');
    await expect(sheet.getByRole('button', { name: '完成' })).toBeInViewport({ ratio: 1 });
    await page.getByTestId('migrated-drawer-close').click();
    await expect(sheet).toBeHidden();
    await expect(page.getByRole('button', { name: '重新打开' })).toBeVisible();
  });

  test('migrated DrawerFrame keeps a single visible heading from its content', async ({ page }) => {
    await open(page, 'migrated-drawer-frame', 'normal');
    const sheet = page.getByRole('dialog', { name: '小雨' });
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole('heading', { name: '小雨' })).toHaveCount(2); // sr-only title + visible h2
    await expect(sheet.locator('[data-responsive-sheet-part="header"]')).toHaveClass(/sr-only/);
    await page.keyboard.press('Escape');
    await expect(sheet).toBeHidden();
  });

  test('turn details: failed call opens with its cause, other calls expand to request/response, whole turn copies as JSON', async ({ page }) => {
    await page.addInitScript(() => {
      (window as any).__copied = [];
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (text: string) => { (window as any).__copied.push(text); } } });
    });
    await open(page, 'turn-details', 'normal');
    const sheet = page.getByRole('dialog', { name: '本轮详情' });
    await expect(sheet.locator('[data-turn-summary]')).toContainText('共 4 次模型调用，第 3 次失败：模型服务处理本次请求时返回错误（HTTP 503）');
    const rows = sheet.locator('.replica-task-result-list > li');
    await expect(rows).toHaveCount(4);
    // Chronological, although the fixture returns newest first.
    await expect(rows.nth(0)).toContainText('1. 对话思考');
    await expect(rows.nth(2)).toContainText('3. 聊天回复');
    for (const index of [0, 1, 3]) await expect(rows.nth(index).locator('.replica-task-round-trigger')).toHaveAttribute('aria-expanded', 'false');
    const failed = rows.nth(2);
    await expect(failed.locator('.replica-task-round-trigger')).toHaveAttribute('aria-expanded', 'true');
    await expect(failed.getByRole('region', { name: '失败原因' })).toContainText('HTTP 503');
    await expect(failed.getByRole('region', { name: '调用错误' })).toContainText('provider_unavailable');
    // Agent runs: readable names and the failure, ids stay in 技术信息.
    await expect(sheet.getByRole('region', { name: 'Agent 运行' })).toContainText('对话思考');
    await expect(sheet.getByRole('region', { name: 'Agent 运行' })).not.toContainText('conversation_cognition');

    const first = rows.nth(0);
    await first.locator('.replica-task-round-trigger').click();
    await expect(first.locator('.replica-task-round-trigger')).toHaveAttribute('aria-expanded', 'true');
    await first.getByRole('button', { name: /^请求内容/ }).click();
    await expect(first.getByText('今天去看展了吗？')).toBeVisible();
    await first.getByRole('button', { name: /^返回内容/ }).click();
    await expect(first.getByText('去了，人不多，下次一起去。')).toBeVisible();
    await expectNoLayoutDefects(page, sheet, { minTarget: 0 });

    await sheet.getByRole('button', { name: '复制整轮 JSON' }).click();
    await expect(sheet.getByRole('button', { name: '已复制' })).toBeVisible();
    const copied = JSON.parse((await page.evaluate(() => (window as any).__copied as string[]))[0]);
    expect(copied.turnId).toBe('turn-fixture-0001');
    expect(copied.modelRuns.map((row: any) => row.ref)).toEqual(['mr-1', 'mr-2', 'mr-3', 'mr-4']);
    expect(copied.agentRuns).toHaveLength(2);
    expect(copied.modelRuns[2].data).toMatchObject({ status: 'failed', error_code: 'provider_unavailable' });
  });

  test('turn details: read error offers a retry that reads again', async ({ page }) => {
    await open(page, 'turn-details', 'read-error');
    const sheet = page.getByRole('dialog', { name: '本轮详情' });
    await expect(sheet.getByRole('alert')).toContainText('诊断服务暂时没有响应');
    await expect(sheet.getByRole('button', { name: '复制整轮 JSON' })).toBeDisabled();
    const reads = async () => (await events(page)).filter(e => e.action === 'turn-read').length;
    const before = await reads();
    await sheet.getByRole('button', { name: '重新读取本轮记录' }).click();
    await expect.poll(reads).toBeGreaterThan(before);
  });

  test('data and task states expose retry/cancel actions that reach the fixture', async ({ page }) => {
    await open(page, 'record-list', 'error');
    await page.getByRole('button', { name: '重新读取约定' }).click();
    await open(page, 'task-status', 'failed');
    await page.getByRole('button', { name: '重试' }).click();
    await page.getByRole('button', { name: '查看技术详情' }).click();
    await expect(page.getByText('provider_timeout after 120s')).toBeVisible();
    const actions = (await events(page)).map(e => `${e.action}:${e.args[0]}`);
    expect(actions).toContain('retry:task');
    await open(page, 'task-status', 'running');
    await expect(page.getByRole('progressbar', { name: '处理进度' })).toBeVisible();
    await expect(page.locator('[data-task-state="running"]')).not.toHaveAttribute('data-feedback-kind', 'success');
  });
});

test('workspace dialog: phone opens on the title and busy cannot dismiss', {tag:['@patterns','@mobile']}, async({page})=>{
  await open(page,'workspace-dialog','page');
  if(isPhone(page))await expect(page.getByRole('heading',{name:'业务记录'})).toBeFocused();
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeHidden();
  await open(page,'workspace-dialog','busy');
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();
  await expect(page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true})).toHaveCount(0);
});

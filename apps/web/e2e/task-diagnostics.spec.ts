import { expect, test, type Page } from '@playwright/test';
import { expectNoLayoutDefects } from './support/layout';

// Task detail and model-call diagnostics in the real app (real routes, pages,
// components and styles) with the business API answered at the fetch boundary
// by the scenario preview mode (VITE_API_SCENARIOS=1, src/v3/scenarios).
const SCENARIO_ORIGIN = `http://127.0.0.1:${Number(process.env.UI_SCENARIO_PORT || 5198)}`;
const isPhone = (page: Page) => page.viewportSize()!.width < 640;

async function openScenario(page: Page, scenario: string) {
  await page.goto(`${SCENARIO_ORIGIN}/?scenario=${scenario}`);
  // First load of a cold dev server transforms the module graph; allow for it.
  await expect(page.locator('[data-api-scenarios]')).toBeVisible({ timeout: 30_000 });
}
async function openTask(page: Page, scenario: string) {
  await openScenario(page, scenario);
  const dialog = page.getByRole('dialog').last();
  await expect(dialog.locator('.replica-task-overview')).toBeVisible({ timeout: 20_000 });
  await expect.poll(() => dialog.evaluate(el => el.getAnimations().filter(a => a.playState === 'running').length)).toBe(0);
  return dialog;
}
async function inViewport(page: Page, locator: ReturnType<Page['locator']>) {
  const box = (await locator.boundingBox())!;
  return box.y >= 0 && box.y + box.height <= page.viewportSize()!.height;
}

test.describe('task detail and diagnostics (scenario preview)', { tag: ['@task-diagnostics'] }, () => {
  test('failed task: status and cause first, progress by default, calls separate, retry reachable', async ({ page }) => {
    const dialog = await openTask(page, 'task-failed');
    const overview = dialog.locator('.replica-task-overview');
    await expect(overview).toContainText('需要处理');
    await expect(overview).toContainText('HTTP 503');
    await expect(overview).not.toContainText(/已尝试|模型调用|次失败/);
    await expect(dialog.getByRole('tab', { name: '执行阶段' })).toHaveAttribute('data-state', 'active');
    await expect(dialog.locator('.replica-task-history > li')).toHaveCount(4);
    const retry = dialog.getByRole('button', { name: '重试任务' });
    expect(await inViewport(page, retry)).toBe(true);

    await dialog.getByRole('tab', { name: '模型调用' }).click();
    await expect(dialog.locator('.replica-task-calls .replica-task-section-head')).toHaveText('模型调用3 次 · 2 次失败');
    const rows = dialog.locator('.replica-task-result-list > li');
    await expect(rows.nth(0).locator('.replica-task-round-trigger')).toHaveAttribute('aria-expanded', 'false');
    await expect(rows.nth(1).locator('.replica-task-round-trigger')).toHaveAttribute('aria-expanded', 'true');
    await expect(rows.nth(1).locator('.replica-task-round-error')).toContainText('HTTP 503');
    // Raw input stays closed until asked for.
    await expect(rows.nth(1).locator('.replica-raw-content-trigger').first()).toHaveAttribute('aria-expanded', 'false');
    await expectNoLayoutDefects(page, dialog, { minTarget: 0 });
  });

  test('retry submits through the real workflow command and the summary reflects the new state', async ({ page }) => {
    const dialog = await openTask(page, 'task-failed');
    const trigger = dialog.getByRole('button', { name: '重试任务' });
    await trigger.click();
    const confirm = page.getByRole('dialog').last();
    const submit = confirm.getByRole('button', { name: '确认重试任务' });
    await expect(submit).toBeDisabled();
    await confirm.locator('textarea').fill('模型服务已恢复');
    await submit.click();
    await expect(dialog.locator('.replica-task-overview')).toContainText('等待开始');
    await expect(dialog.locator('.replica-task-overview')).toContainText('已受理，尚未开始执行');
    await expect(dialog.locator('.replica-task-history > li')).toHaveCount(5);
  });

  test('refresh re-reads status: running stays running until the task itself completes', async ({ page }) => {
    const dialog = await openTask(page, 'task-running');
    const overview = dialog.locator('.replica-task-overview');
    await expect(overview).toContainText('进行中');
    await expect(overview).toContainText('完成前不会显示为已完成');
    await expect(overview).not.toContainText('运行超过 15 分钟');
    // One successful model call does not complete the task.
    await dialog.getByRole('tab', { name: '模型调用' }).click();
    await expect(dialog.locator('.replica-task-result-list > li')).toHaveCount(1);
    await expect(overview).toContainText('进行中');
    const refresh = dialog.getByRole('button', { name: '刷新任务状态' });
    await refresh.click();
    await expect(overview).toContainText('进行中');
    await refresh.click();
    await expect(overview).toContainText('已完成');
    await expect(dialog.locator('.replica-task-result-list > li')).toHaveCount(2);
  });

  test('task read failure keeps a retry and recovers', async ({ page }) => {
    await openScenario(page, 'task-read-error');
    const dialog = page.getByRole('dialog').last();
    await expect(dialog).toContainText('任务暂时无法读取', { timeout: 20_000 });
    await dialog.getByRole('button', { name: '重新读取任务' }).click();
    await expect(dialog.locator('.replica-task-overview')).toContainText('需要处理');
  });

  test('model-call read failure keeps a retry and recovers', async ({ page }) => {
    const dialog = await openTask(page, 'calls-read-error');
    await dialog.getByRole('tab', { name: '模型调用' }).click();
    await dialog.getByRole('button', { name: '重新读取模型调用' }).click();
    await expect(dialog.locator('.replica-task-result-list > li')).toHaveCount(3);
  });

  test('slow reads show progress on the refresh control while keeping the confirmed state', async ({ page }) => {
    test.setTimeout(90_000); // every request in this scenario is delayed by 2.5 s
    const dialog = await openTask(page, 'task-slow');
    await dialog.getByRole('button', { name: '刷新任务状态' }).click();
    await expect(dialog.getByRole('button', { name: '正在刷新…' })).toBeDisabled();
    await expect(dialog.locator('.replica-task-overview')).toContainText('HTTP 503');
    await expect(dialog.getByRole('button', { name: '刷新任务状态' })).toBeEnabled({ timeout: 10_000 });
  });

  test('very long input stays bounded and the footer action stays reachable', async ({ page }) => {
    const dialog = await openTask(page, 'task-long');
    await dialog.getByRole('tab', { name: '模型调用' }).click();
    await dialog.locator('.replica-raw-content-trigger', { hasText: '输入' }).first().click();
    const body = dialog.locator('.replica-raw-content-body').first();
    await expect(body).toBeVisible();
    const limit = Math.min(page.viewportSize()!.height * 0.5, 420) + 1;
    expect((await body.boundingBox())!.height).toBeLessThanOrEqual(limit);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(await inViewport(page, dialog.getByRole('button', { name: '重试任务' }))).toBe(true);
  });

  test('call detail: error and actions before payloads, payloads open on demand and copy, focus returns', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: SCENARIO_ORIGIN });
    await openScenario(page, 'calls-long');
    const source = page.locator('[data-testid^=diag-open-]').first();
    await source.click();
    const dialog = page.getByRole('dialog').last();
    await expect(dialog).toContainText('调用错误');
    const actions = dialog.locator('.replica-model-call-actions');
    const payload = dialog.locator('.replica-raw-content').first();
    expect((await actions.boundingBox())!.y).toBeLessThan((await payload.boundingBox())!.y);
    await expect(payload.locator('.replica-raw-content-trigger')).toHaveAttribute('aria-expanded', 'false');
    await payload.getByRole('button', { name: '复制请求内容' }).click();
    await expect(payload.getByRole('button', { name: '复制请求内容' })).toContainText('已复制');
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('完整提示词 第 1 段');
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(source).toBeFocused();
  });

  test('media task: refresh is named for what it does (re-read status, no regeneration)', async ({ page }) => {
    const dialog = await openTask(page, 'task-media');
    await expect(dialog.locator('.replica-task-stages > li')).toHaveCount(3);
    await expect(dialog).toContainText('刷新只读取最新状态，不会重新生成');
    const refresh = dialog.getByRole('button', { name: '刷新媒体状态' });
    await expect(refresh).toBeVisible();
    await expect(dialog.getByRole('button', { name: '重新读取媒体任务' })).toHaveCount(0);
    await refresh.click();
    await expect(dialog.getByRole('button', { name: '刷新媒体状态' })).toBeEnabled();
    await expect(dialog.locator('.replica-task-stages > li')).toHaveCount(3);
  });

  test('scenario switch reloads into the chosen state without leftovers; mock mode is labelled', async ({ page }) => {
    await openTask(page, 'task-failed');
    const badge = page.locator('[data-api-scenarios]');
    await expect(badge).toContainText('预览模式');
    await badge.locator('summary').click();
    await badge.getByRole('combobox', { name: '选择预览场景' }).selectOption('task-succeeded');
    await expect(page).toHaveURL(/scenario=task-succeeded/);
    const dialog = page.getByRole('dialog').last();
    await expect(dialog.locator('.replica-task-overview')).toContainText('已完成');
    await expect(dialog.locator('.replica-task-overview')).not.toContainText('需要处理');
    if (!isPhone(page)) await expectNoLayoutDefects(page, dialog, { minTarget: 0 });
  });
});

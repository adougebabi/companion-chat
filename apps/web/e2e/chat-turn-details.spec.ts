import { expect, test, type Page } from '@playwright/test';
import { expectNoLayoutDefects } from './support/layout';

// Chat 本轮详情 in the real app (ChatPage, ChatTurnFeedback, TurnDetailsDrawer)
// with the conversation and owner diagnostic records answered at the fetch
// boundary by the scenario preview mode (VITE_API_SCENARIOS=1, src/v3/scenarios).
const SCENARIO_ORIGIN = `http://127.0.0.1:${Number(process.env.UI_SCENARIO_PORT || 5198)}`;
const isPhone = (page: Page) => page.viewportSize()!.width < 640;

async function openChat(page: Page) {
  await page.goto(`${SCENARIO_ORIGIN}/?scenario=chat-turn-details`);
  await expect(page.locator('[data-api-scenarios]')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('去了，人不多，下次一起去。')).toBeVisible({ timeout: 20_000 });
}

test.describe('chat turn details (scenario preview)', { tag: ['@chat-turn'] }, () => {
  test('a successful reply opens its turn in one click; closing returns focus to the message action', async ({ page }) => {
    await openChat(page);
    const trigger = page.getByRole('button', { name: '本轮详情' }).and(page.locator('[data-turn-details-trigger="turn-ok"]'));
    await expect(trigger).toBeVisible();
    // Only assistant replies carry the per-message action; the user's own messages do not.
    await expect(page.locator('[data-turn-details-trigger]')).toHaveCount(1);
    // The action sits in the reply's header; on phones it keeps a 40px hit area.
    await expectNoLayoutDefects(page, page.locator('.companion-message').filter({ has: trigger }), { minTarget: isPhone(page) ? 40 : 0 });
    if (isPhone(page)) expect((await trigger.boundingBox())!.height).toBeGreaterThanOrEqual(40);

    await trigger.click();
    const sheet = page.getByRole('dialog', { name: '本轮详情' });
    await expect(sheet.locator('[data-turn-summary]')).toContainText('共 2 次模型调用，全部成功');
    const rows = sheet.locator('.replica-task-result-list > li');
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0)).toContainText('对话思考');
    await expect(rows.nth(1)).toContainText('聊天回复');
    // Nothing failed, so nothing opens by itself.
    await expect(sheet.locator('.replica-task-round-trigger[aria-expanded="true"]')).toHaveCount(0);
    await expectNoLayoutDefects(page, sheet, { minTarget: isPhone(page) ? 40 : 0 });

    await sheet.getByRole('button', { name: '关闭', exact: true }).last().click();
    await expect(sheet).toBeHidden();
    await expect(trigger).toBeFocused();
  });

  test('a failed turn opens the same drawer from its feedback with the failed call expanded; retry is unchanged', async ({ page }) => {
    await openChat(page);
    const feedback = page.locator('[data-feedback-kind="error"]').filter({ hasText: '暂时没能回应' });
    await expect(feedback.getByRole('button', { name: '重试原回应' })).toBeVisible();
    await feedback.getByRole('button', { name: '本轮详情' }).click();

    const sheet = page.getByRole('dialog', { name: '本轮详情' });
    await expect(sheet.getByRole('region', { name: '本次回应' })).toContainText('本次回复未发送');
    await expect(sheet.locator('[data-turn-summary]')).toContainText('共 2 次模型调用，第 2 次失败：模型服务处理本次请求时返回错误（HTTP 503）');
    const failed = sheet.locator('.replica-task-result-list > li[data-call-status="failure"]');
    await expect(failed.locator('.replica-task-round-trigger')).toHaveAttribute('aria-expanded', 'true');
    await expect(failed.getByRole('region', { name: '调用错误' })).toContainText('provider_unavailable');
    await expect(sheet.getByRole('region', { name: 'Agent 运行' })).toContainText('执行失败');

    const first = sheet.locator('.replica-task-result-list > li').first();
    await first.locator('.replica-task-round-trigger').click();
    await first.getByRole('button', { name: /^返回内容/ }).click();
    await expect(first.getByText('她想答应，但要先看看周末的排班。')).toBeVisible();
    // Internal ids only inside the collapsed 技术信息.
    await expect(sheet.getByText('turn:turn-failed')).toBeHidden();
    await sheet.getByRole('button', { name: '技术信息' }).click();
    await expect(sheet.getByText('turn:turn-failed').first()).toBeVisible();
    await expectNoLayoutDefects(page, sheet, { minTarget: 0 });
  });
});

import { expect, test } from '@playwright/test';
import { expectNoLayoutDefects } from './support/layout';

// 通知与待处理 (#/inbox) — the only in-app route that renders BrowsePage — with
// /api/notifications, /api/pending and read-all answered at the fetch boundary by
// the scenario preview mode (VITE_API_SCENARIOS=1, src/v3/scenarios), in the
// shapes of Core's notification_center.go and pending_center.go.
const SCENARIO_ORIGIN = `http://127.0.0.1:${Number(process.env.UI_SCENARIO_PORT || 5198)}`;

test.describe('inbox: notifications and pending (scenario preview)', { tag: ['@inbox'] }, () => {
  test('shows the linked reply and the failed task, marks all read once, opens the original task', async ({ page }) => {
    await page.goto(`${SCENARIO_ORIGIN}/?scenario=inbox`);
    await expect(page.locator('[data-api-scenarios]')).toBeVisible({ timeout: 30_000 });
    const main = page.getByRole('main');
    await expect(main.getByRole('heading', { name: '林夏 · 新回复' })).toBeVisible({ timeout: 20_000 });
    // The preview is the exact linked message, read through the conversation API.
    await expect(main).toContainText('今天的画画完了，傍晚的海很好看，回头给你看。');
    await expect(main.getByRole('tab', { name: '未读 1' })).toBeVisible();
    const failed = main.getByRole('button', { name: /独立目标规划/ });
    await expect(failed).toContainText('执行失败');
    await expectNoLayoutDefects(page, main, { minTarget: 0 });

    await main.getByRole('button', { name: '全部标为已读' }).click();
    await expect(main.getByRole('tab', { name: '未读 0' })).toBeVisible();
    await expect(main.getByRole('button', { name: '标为已读', exact: true })).toHaveCount(0);
    // Marking read does not resolve the pending item.
    await expect(failed).toBeVisible();

    await failed.click();
    const dialog = page.getByRole('dialog').last();
    await expect(dialog.locator('.replica-task-overview')).toContainText('需要处理', { timeout: 20_000 });
  });
});

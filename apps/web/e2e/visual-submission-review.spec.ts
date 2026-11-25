import { expect, test } from '@playwright/test';
import { expectNoLayoutDefects } from './support/layout';

// Media page → 视觉身份 with a candidate submission paused on uncertain provider
// acceptance, answered at the fetch boundary by the scenario preview mode
// (VITE_API_SCENARIOS=1, src/v3/scenarios). The owner must confirm by hand that
// the image comes from the original task before the workflow may resume.
const SCENARIO_ORIGIN = `http://127.0.0.1:${Number(process.env.UI_SCENARIO_PORT || 5198)}`;

test.describe('visual identity submission review (scenario preview)', { tag: ['@visual-identity'] }, () => {
  test('manual review checkbox gates the resume, works from the keyboard and submits once', async ({ page }) => {
    await page.goto(`${SCENARIO_ORIGIN}/?scenario=visual-submission-review`);
    await expect(page.locator('[data-api-scenarios]')).toBeVisible({ timeout: 30_000 });
    const open = page.getByRole('button', { name: '核对后恢复已有候选图' });
    await expect(open).toBeVisible({ timeout: 20_000 });
    await open.click();

    const dialog = page.getByRole('dialog', { name: '核对后恢复候选图' });
    const reviewed = dialog.getByRole('checkbox', { name: '我已人工核对原生成任务与同源图片' });
    const submit = dialog.getByRole('button', { name: '确认核对并恢复原任务' });
    await expect(reviewed).toHaveAttribute('data-state', 'unchecked');
    await expect.poll(() => dialog.evaluate(el => el.getAnimations().filter(a => a.playState === 'running').length)).toBe(0);
    await expectNoLayoutDefects(page, dialog, { minTarget: 0 });

    // A reason alone is not enough: the explicit manual confirmation is required.
    await dialog.getByRole('textbox', { name: '核对说明' }).fill('已核对候选图来自原任务');
    await submit.click();
    await expect(page.getByText('请明确确认已人工核对原生成任务与同源图片。').first()).toBeVisible();
    await expect(dialog).toBeVisible();

    await reviewed.focus();
    await page.keyboard.press('Space');
    await expect(reviewed).toHaveAttribute('data-state', 'checked');
    await expect(reviewed).toBeFocused();
    // The visible label toggles it as well.
    await dialog.getByText('我已人工核对原生成任务与同源图片').click();
    await expect(reviewed).toHaveAttribute('data-state', 'unchecked');
    await page.keyboard.press('Space');
    await expect(reviewed).toHaveAttribute('data-state', 'checked');

    await submit.click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByText('原图片结果已核对，原工作流恢复请求已发送').first()).toBeVisible();
    await expect(open).toHaveCount(0);
  });
});

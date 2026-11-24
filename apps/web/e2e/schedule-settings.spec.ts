import { expect, test, type Page } from '@playwright/test';
import { expectNoLayoutDefects, expectTabsReachable } from './support/layout';

// 「约定与日程设置」 in real Chromium at phone / tablet / desktop widths.
// Data comes from ui-lab/fixtures/schedule-settings.ts; the clock is frozen.
const FIXED_NOW = new Date('2026-11-16T10:30:00+08:00');
const TABS = ['当天安排', '接下来几天', '意向', '约定', '生活倾向'] as const;

async function open(page: Page, query = 'section=today') {
  await page.clock.setFixedTime(FIXED_NOW);
  await page.goto(`/ui-lab/?fixture=schedule-settings&${query}`);
  const sheet = page.getByRole('dialog', { name: '约定与日程设置' });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole('tablist', { name: '日程分区' })).toBeVisible();
  // Wait for the open animation to finish so geometry is final.
  await expect.poll(() => sheet.evaluate(el => el.getAnimations().filter(a => a.playState === 'running').length)).toBe(0);
  return sheet;
}
const events = (page: Page) => page.evaluate(() => (window as any).__uiLabEvents as { action: string; args: any[] }[]);
const isPhone = (page: Page) => page.viewportSize()!.width < 640;
const minTarget = (page: Page) => (isPhone(page) ? 40 : 0);

async function shot(page: Page, name: string) {
  await page.screenshot({ path: test.info().outputPath(`${name}.png`) });
}

test.describe('约定与日程设置 sheet', { tag: ['@schedule', '@overlay'] }, () => {
  test('geometry: full width on phones, side sheet on wider screens; no overflow', async ({ page }) => {
    const sheet = await open(page);
    const vw = page.viewportSize()!.width, vh = page.viewportSize()!.height;
    const box = (await sheet.boundingBox())!;
    if (isPhone(page)) {
      expect(box.width).toBeCloseTo(vw, 0);
      expect(box.x).toBeCloseTo(0, 0);
    } else {
      expect(box.width).toBeCloseTo(Math.min(540, vw), 0);
      expect(box.x + box.width).toBeCloseTo(vw, 0);
    }
    expect(box.height).toBeCloseTo(vh, 0);
    // The close button is a full touch target and stays inside the sheet.
    const close = sheet.getByRole('button', { name: '关闭' });
    const closeBox = (await close.boundingBox())!;
    expect(closeBox.width).toBeGreaterThanOrEqual(44);
    expect(closeBox.height).toBeGreaterThanOrEqual(44);
    await expectNoLayoutDefects(page, sheet, { minTarget: minTarget(page) });
    await shot(page, 'open-today');
  });

  test('every tab is reachable, switches its panel and lays out without defects', async ({ page }) => {
    const sheet = await open(page);
    const list = sheet.getByRole('tablist', { name: '日程分区' });
    await expectTabsReachable(list);
    for (const label of TABS) {
      const tab = list.getByRole('tab', { name: label });
      await tab.scrollIntoViewIfNeeded();
      await tab.click();
      await expect(tab).toHaveAttribute('aria-selected', 'true');
      // The active tab is fully visible after switching.
      const [t, l] = [(await tab.boundingBox())!, (await list.boundingBox())!];
      expect(t.x).toBeGreaterThanOrEqual(l.x - 1);
      expect(t.x + t.width).toBeLessThanOrEqual(l.x + l.width + 1);
      await expectNoLayoutDefects(page, sheet, { minTarget: minTarget(page) });
      await shot(page, `tab-${TABS.indexOf(label)}`);
    }
    await expect(sheet.getByText('早上喜欢先喝温水再看书')).toBeVisible();
  });

  test('date selection queries the chosen day and keeps the sheet open', async ({ page }) => {
    const sheet = await open(page);
    await sheet.getByRole('combobox', { name: '查看日期' }).click();
    const calendar = page.locator('[data-slot="popover-content"]');
    await expect(calendar).toBeVisible();
    await expectNoLayoutDefects(page, calendar);
    await shot(page, 'date-picker');
    await calendar.getByRole('button', { name: /18/ }).first().click();
    await expect(sheet.getByRole('combobox', { name: '查看日期' })).toHaveText(/2026-11-18/);
    await sheet.getByRole('button', { name: '查找' }).click();
    const reopened = page.getByRole('dialog', { name: '约定与日程设置' });
    await expect(reopened.getByRole('heading', { name: '2026-11-18' })).toBeVisible();
    expect((await events(page)).some(e => e.action === 'schedules' && e.args[1]?.from === '2026-11-18' && e.args[1]?.to === '2026-11-18')).toBe(true);
    await expect(reopened.getByRole('tab', { name: '当天安排' })).toHaveAttribute('aria-selected', 'true');
    await reopened.getByRole('button', { name: '回到今天' }).click();
    await expect(page.getByRole('dialog', { name: '约定与日程设置' }).getByRole('heading', { name: '2026-11-16' })).toBeVisible();
  });

  test('actions: complete schedule, suggestion, invitation and promise flows stay usable', async ({ page }) => {
    const sheet = await open(page);
    await expect(sheet.getByRole('link', { name: '到聊天中发起邀请' })).toHaveAttribute('href', '#/c/A/chat');

    // 日程设置 → 补全日程 opens the existing-record notice (data is preserved).
    await sheet.getByRole('button', { name: '日程设置' }).click();
    const nested = page.getByRole('dialog', { name: '日程设置', exact: true });
    await expect(nested).toBeVisible();
    await expectNoLayoutDefects(page, nested, { minTarget: minTarget(page) });
    await nested.getByRole('button', { name: '补全日程' }).click();
    const notice = page.getByRole('dialog', { name: '这一天已有日程记录' });
    await expect(notice).toBeVisible();
    await expect(notice).toContainText('包含 3 项安排');
    await expectNoLayoutDefects(page, notice, { minTarget: minTarget(page) });
    await shot(page, 'complete-schedule-notice');
    await page.keyboard.press('Escape');
    await expect(notice).toBeHidden();
    await page.keyboard.press('Escape');
    await expect(nested).toBeHidden();

    // 调整建议 → 填写日程建议 opens the suggestion form; its submit stays reachable.
    await sheet.getByRole('button', { name: '调整建议' }).click();
    await page.getByRole('dialog', { name: '调整建议', exact: true }).getByRole('button', { name: '填写日程建议' }).click();
    const form = page.getByRole('dialog', { name: '建议一项日程' });
    await expect(form).toBeVisible();
    await expectNoLayoutDefects(page, form, { minTarget: minTarget(page) });
    // The legacy drawer footer is pinned: primary action visible without
    // scrolling, footer flush with the sheet's bottom edge.
    const submit = form.getByRole('button', { name: '发出建议' });
    await expect(submit).toBeInViewport({ ratio: 1 });
    const footer = (await form.locator('.domain-overlay-content > footer').boundingBox())!;
    const formBox = (await form.boundingBox())!;
    expect(Math.abs(footer.y + footer.height - (formBox.y + formBox.height))).toBeLessThanOrEqual(1);
    await shot(page, 'suggestion-form');
    await form.locator(':scope > [data-slot="sheet-close"]').click();
    await expect(form).toBeHidden();

    // 约定 → 和她约定 / 查看约定.
    await page.keyboard.press('Escape');
    await sheet.getByRole('tab', { name: '约定' }).click();
    await expect(sheet.getByText('周六一起去看展')).toBeVisible();
    await sheet.getByRole('button', { name: '查看约定' }).first().click();
    const detail = page.getByRole('dialog', { name: '约定与履行证据' });
    await expect(detail).toBeVisible();
    await expect(detail.getByRole('heading', { name: '周六一起去看展' })).toBeVisible();
    await expectNoLayoutDefects(page, detail, { minTarget: minTarget(page) });
    expect((await events(page)).some(e => e.action === 'readPromise' && e.args[0] === 'promise-1')).toBe(true);
    await detail.locator('footer').getByRole('button', { name: '取消', exact: true }).click();
    await expect(detail).toBeHidden();
    await sheet.getByRole('button', { name: '和她约定' }).click();
    const propose = page.getByRole('dialog').filter({ has: page.getByRole('textbox').first() }).last();
    await expect(propose).toBeVisible();
    await expectNoLayoutDefects(page, propose, { minTarget: minTarget(page) });
    await shot(page, 'promise-form');
  });

  test('long content scrolls inside the body; header and close stay in view', async ({ page }) => {
    const sheet = await open(page, 'section=today&variant=long');
    const body = sheet.locator('[data-responsive-sheet-part="body"]');
    await sheet.getByRole('button', { name: /在旧街尽头/ }).click();
    await sheet.getByRole('button', { name: /去花店/ }).click();
    await expectNoLayoutDefects(page, sheet, { minTarget: minTarget(page) });
    const scrollable = await body.evaluate(el => el.scrollHeight > el.clientHeight);
    if (scrollable) {
      await body.evaluate(el => el.scrollTo(0, el.scrollHeight));
      await expect(sheet.getByRole('button', { name: '关闭' })).toBeInViewport();
      await expect(sheet.getByRole('heading', { name: '约定与日程设置' })).toBeInViewport();
    }
    await shot(page, 'long-content');
  });

  test('closing by Escape and close button, then reopening keeps the selected tab', async ({ page }) => {
    const sheet = await open(page, 'section=promises');
    await expect(sheet.getByRole('tab', { name: '约定' })).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('Escape');
    await expect(sheet).toBeHidden();
    const trigger = page.getByRole('button', { name: '约定与日程设置' });
    await expect(trigger).toBeFocused();
    await trigger.click();
    await expect(sheet.getByRole('tab', { name: '约定' })).toHaveAttribute('aria-selected', 'true');
    await sheet.getByRole('button', { name: '关闭' }).click();
    await expect(sheet).toBeHidden();
    await expectNoLayoutDefects(page, page.locator('[data-ui-lab-page]'));
    await shot(page, 'closed');
  });

  test('empty data renders empty states without layout defects', async ({ page }) => {
    const sheet = await open(page, 'section=today&variant=empty');
    await expect(sheet.getByText('这一天尚无已接受日程')).toBeVisible();
    await expectNoLayoutDefects(page, sheet, { minTarget: minTarget(page) });
    await sheet.getByRole('tab', { name: '约定' }).click();
    await expect(sheet.getByText(/还没有约定提议/)).toBeVisible();
    await shot(page, 'empty');
  });
});

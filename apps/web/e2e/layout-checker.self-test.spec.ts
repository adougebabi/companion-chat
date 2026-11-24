import { expect, test } from '@playwright/test';
import { findLayoutDefects } from './support/layout';

// The gate must FAIL on known-bad layouts. If any of these stop reporting a
// defect, the checker has lost its teeth and the UI gate must not be trusted.

test('detects clipped controls, page overflow and spilled button text', async ({ page }) => {
  await page.setContent(`<!doctype html><meta name=viewport content="width=device-width">
    <body style="margin:0">
      <section id=box style="width:200px;overflow:hidden;display:flex;gap:8px">
        <button style="flex:none;width:120px">一</button><button style="flex:none;width:120px">被裁切</button>
      </section>
      <div style="width:calc(100vw + 40px);height:4px"></div>
      <section id=fixed style="width:120px"><button style="height:24px;width:100px;white-space:normal">很长很长很长很长很长的按钮文字</button></section>
    </body>`);
  const clipped = await findLayoutDefects(page, page.locator('#box'));
  expect(clipped.map(d => d.kind)).toContain('control-clipped');
  expect(clipped.map(d => d.kind)).toContain('page-overflow');
  const spilled = await findLayoutDefects(page, page.locator('#fixed'));
  expect(spilled.map(d => d.kind)).toContain('content-overflow');
});

test('re-introducing the old schedule-sheet CSS is reported as a defect', async ({ page }) => {
  test.skip(page.viewportSize()!.width >= 640, 'the regression is phone-only');
  await page.clock.setFixedTime(new Date('2026-11-16T10:30:00+08:00'));
  await page.goto('/ui-lab/?fixture=schedule-settings&section=today');
  const sheet = page.getByRole('dialog', { name: '约定与日程设置' });
  await expect(sheet).toBeVisible();
  // The pre-fix rules: 75vw business width override and fixed-height triggers.
  await page.addStyleTag({ content: `
    .product-sheet.schedule-settings-sheet[data-responsive-sheet]{width:75vw!important}
    [data-domain-disclosure] [data-slot=collapsible-trigger]{height:36px!important}
    [data-responsive-sheet] .domain-rich-label > .schedule-time{flex-basis:auto!important}` });
  await page.waitForTimeout(300);
  const defects = await findLayoutDefects(page, sheet, { minTarget: 40 });
  expect(defects.length, JSON.stringify(defects, null, 1)).toBeGreaterThan(0);
  expect(defects.map(d => d.kind)).toContain('content-overflow');
});

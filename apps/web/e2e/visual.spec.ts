import { expect, test, type Page } from '@playwright/test';
import { GALLERY_CATALOG } from '../ui-lab/fixtures/gallery-catalog';
import { verifyBaselines } from './support/baselines.mjs';

// Approved visual baselines for shared components and key business states.
// Missing or out-of-band-modified baselines are UNVERIFIED (never a pass);
// see scripts/ui-gate.mjs, which reports that status distinctly.
const FIXED_NOW = new Date('2026-11-16T10:30:00+08:00');

test.beforeAll(() => {
  if (process.env.UI_BASELINE_APPROVAL_REASON) return;
  const store = verifyBaselines();
  if (store.status !== 'ok') throw new Error(`UI_BASELINE_UNVERIFIED: ${store.detail}`);
});

async function settle(page: Page) {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity || (a.timeline !== null && a.timeline !== document.timeline)));
}

for (const [id, entry] of Object.entries(GALLERY_CATALOG)) {
  test.describe(`visual: ${entry.title}`, { tag: ['@visual', '@patterns', ...(id.startsWith('migrated-') ? ['@overlay'] : [])] }, () => {
    for (const state of entry.states) {
      test(`${id} ${state}`, async ({ page }) => {
        await page.clock.setFixedTime(FIXED_NOW);
        await page.goto(`/ui-lab/?fixture=gallery&component=${id}&state=${state}`);
        await expect(page.locator(`[data-gallery-target][data-state="${state}"]`)).toBeAttached();
        if (entry.overlay) await expect(page.getByRole('dialog')).toBeVisible();
        await settle(page);
        const target = entry.overlay ? page : page.locator('[data-gallery-target]');
        await expect(target).toHaveScreenshot(`${id}--${state}.png`);
      });
    }
  });
}

// [name, query, text that proves the async fixture content has rendered]
const SCHEDULE_STATES = [
  ['today', 'section=today', '和朋友视频通话'],
  ['promises', 'section=promises', '每天睡前道晚安'],
  ['habits', 'section=habits', '早上喜欢先喝温水再看书'],
  ['long', 'section=today&variant=long', '和朋友视频通话'],
  ['empty', 'section=today&variant=empty', '这一天尚无已接受日程'],
] as const;

test.describe('visual: 约定与日程设置', { tag: ['@visual', '@schedule', '@overlay'] }, () => {
  for (const [name, query, ready] of SCHEDULE_STATES) {
    test(`schedule-settings ${name}`, async ({ page }) => {
      await page.clock.setFixedTime(FIXED_NOW);
      await page.goto(`/ui-lab/?fixture=schedule-settings&${query}`);
      await expect(page.getByRole('dialog', { name: '约定与日程设置' }).getByText(ready).filter({ visible: true }).first()).toBeVisible();
      await settle(page);
      await expect(page).toHaveScreenshot(`schedule-settings--${name}.png`);
    });
  }
});

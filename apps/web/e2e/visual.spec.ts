import { expect, test, type Page } from '@playwright/test';
import { GALLERY_CATALOG } from '../ui-lab/fixtures/gallery-catalog';
import { readManifest, verifyBaselines } from './support/baselines.mjs';
import { expectNoLayoutDefects } from './support/layout';

// Every shared pattern × every applicable state × phone/tablet/desktop in real
// Chromium, opened once: layout checks first, then the approved-baseline
// screenshot. Missing or out-of-band-modified baselines are UNVERIFIED (never a
// pass); see scripts/ui-gate.mjs, which reports that status distinctly. Layout
// defects are checked before the baseline store, so they always fail as such.
const FIXED_NOW = new Date('2026-11-16T10:30:00+08:00');
const isPhone = (page: Page) => page.viewportSize()!.width < 640;

// A screenshot with no approved baseline (e.g. a newly added UI Lab state) is
// UNVERIFIED until a design review approves it — never a pass, and reported
// distinctly from a real visual difference.
let store: ReturnType<typeof verifyBaselines> | undefined;
function requireApprovedBaseline(name: string) {
  if (process.env.UI_BASELINE_APPROVAL_REASON) return;
  store ??= verifyBaselines();
  if (store.status !== 'ok') throw new Error(`UI_BASELINE_UNVERIFIED: ${store.detail}`);
  const file = `${test.info().project.name}/${name}`;
  if (!readManifest()?.files?.[file]) throw new Error(`UI_BASELINE_UNVERIFIED: 缺少已审核的视觉基准 ${file}（需设计审核后经 ui:baselines approve 录入）`);
}

async function settle(page: Page) {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity || (a.timeline !== null && a.timeline !== document.timeline)));
}

for (const [id, entry] of Object.entries(GALLERY_CATALOG)) {
  test.describe(`visual: ${entry.title}`, { tag: ['@visual', '@patterns', ...(id.startsWith('migrated-') ? ['@overlay'] : []), ...(id==='diagnostic-loops'?['@task-diagnostics']:[]), ...(id==='turn-details'?['@chat-turn']:[])] }, () => {
    for (const state of entry.states) {
      test(`${id} ${state}`, async ({ page }) => {
        await page.clock.setFixedTime(FIXED_NOW);
        await page.goto(`/ui-lab/?fixture=gallery&component=${id}&state=${state}`);
        await expect(page.locator(`[data-gallery-target][data-state="${state}"]`)).toBeAttached();
        if (entry.overlay) await expect(page.getByRole('dialog')).toBeVisible();
        // Layout is measured at rest: a sheet still sliding in sits past the edge.
        await settle(page);
        const minTarget = isPhone(page) ? 40 : 0;
        await expectNoLayoutDefects(page, page.locator('[data-ui-lab-page]'), { minTarget });
        if (entry.overlay) {
          const sheet = page.getByRole('dialog');
          const vw = page.viewportSize()!.width;
          const box = (await sheet.boundingBox())!;
          if (isPhone(page) && (id === 'compact-dialog' || id === 'workspace-dialog' && state === 'dialog')) {
            expect(box.width).toBeCloseTo(vw - 36, 0);
            expect(box.x).toBeCloseTo(18, 0);
          } else if (!isPhone(page) && id === 'workspace-dialog') {
            expect(box.x + box.width / 2).toBeCloseTo(vw / 2, 0);
          } else {
            expect(box.x + box.width).toBeCloseTo(vw, 0);
            if (isPhone(page)) expect(box.width).toBeCloseTo(vw, 0);
          }
          if (isPhone(page) && (id === 'bottom-sheet' || id === 'workspace-dialog' && state === 'sheet')) {
            expect(box.y + box.height).toBeCloseTo(page.viewportSize()!.height, 0);
            expect(box.height).toBeLessThanOrEqual(page.viewportSize()!.height * .88 + 1);
          }
          await expectNoLayoutDefects(page, sheet, { minTarget });
          // Footer actions are reachable without scrolling the body.
          const footer = sheet.locator('[data-responsive-sheet-part="footer"], [data-slot="dialog-footer"]');
          if (await footer.count()) await expect(footer).toBeInViewport({ ratio: 1 });
        }
        const target = entry.overlay ? page : page.locator('[data-gallery-target]');
        requireApprovedBaseline(`${id}--${state}.png`);
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
      requireApprovedBaseline(`schedule-settings--${name}.png`);
      await expect(page).toHaveScreenshot(`schedule-settings--${name}.png`);
    });
  }
});

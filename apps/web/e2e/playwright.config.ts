import { defineConfig, devices } from '@playwright/test';
import { baselineDir } from './support/baselines.mjs';

// Real-browser UI acceptance. Chromium only; never replaced by jsdom checks.
// Screenshots go to test-results/; visual baselines to <repo>/.ui-baselines/.
// Both are git-ignored: images are never committed (AGENTS.md).
const port = Number(process.env.UI_LAB_PORT || 5199);
const desktopChrome = devices['Desktop Chrome'];

// Baselines change only through `pnpm ui:baselines approve --reason …`, which
// sets UI_BASELINE_APPROVAL_REASON. A failing test never re-records itself and
// a missing baseline is a failure (UNVERIFIED), not an automatic recording.
const approving = !!process.env.UI_BASELINE_APPROVAL_REASON;
if (!approving && process.argv.some(arg => arg === '-u' || arg.startsWith('--update-snapshots')))
  throw new Error('禁止直接 --update-snapshots：视觉基准只能经设计审核后用 `pnpm ui:baselines approve --reason "<变更与审核说明>"` 更新。');

export default defineConfig({
  testDir: '.',
  outputDir: '../test-results/ui',
  snapshotPathTemplate: `${baselineDir()}/{projectName}/{arg}{ext}`,
  updateSnapshots: approving ? 'all' : 'none',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [['list'], ['html', { outputFolder: '../playwright-report', open: 'never' }]] : [['list']],
  expect: {
    // Same-environment baselines render deterministically: no pixel budget.
    // A ratio tolerance (e.g. 0.2%) let a global radius change pass unnoticed.
    toHaveScreenshot: { animations: 'disabled', caret: 'hide', scale: 'css', maxDiffPixels: 0, threshold: 0.2 },
  },
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    browserName: 'chromium',
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'phone-390', use: { ...desktopChrome, viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, deviceScaleFactor: 2 } },
    { name: 'tablet-768', use: { ...desktopChrome, viewport: { width: 768, height: 1024 }, hasTouch: true } },
    { name: 'desktop-1440', use: { ...desktopChrome, viewport: { width: 1440, height: 900 } } },
  ],
  webServer: {
    command: `node ./node_modules/vite/bin/vite.js --port ${port} --strictPort --host 127.0.0.1`,
    cwd: '..',
    url: `http://127.0.0.1:${port}/ui-lab/`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});

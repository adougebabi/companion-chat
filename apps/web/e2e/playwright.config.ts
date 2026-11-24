import { defineConfig, devices } from '@playwright/test';

// Real-browser UI acceptance. Chromium only; never replaced by jsdom checks.
// Screenshots are written under test-results/ (git-ignored, see AGENTS.md:
// images are never committed).
const port = Number(process.env.UI_LAB_PORT || 5199);
const desktopChrome = devices['Desktop Chrome'];

export default defineConfig({
  testDir: '.',
  outputDir: '../test-results/ui',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [['list'], ['html', { outputFolder: '../playwright-report', open: 'never' }]] : [['list']],
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

import { expect, test, type Page } from '@playwright/test';
import { GALLERY_CATALOG } from '../ui-lab/fixtures/gallery-catalog';
import { expectNoLayoutDefects } from './support/layout';

// Every shared pattern × every applicable state × phone/tablet/desktop in real
// Chromium. Fixture data is fixed; the clock is frozen.
const FIXED_NOW = new Date('2026-11-16T10:30:00+08:00');
const isPhone = (page: Page) => page.viewportSize()!.width < 640;
const events = (page: Page) => page.evaluate(() => (window as any).__uiLabEvents as { action: string; args: any[] }[]);

async function open(page: Page, component: string, state: string) {
  await page.clock.setFixedTime(FIXED_NOW);
  await page.goto(`/ui-lab/?fixture=gallery&component=${component}&state=${state}`);
  await expect(page.locator(`[data-gallery-target][data-state="${state}"]`)).toBeAttached();
}

for (const [id, entry] of Object.entries(GALLERY_CATALOG)) {
  test.describe(`${entry.title} (${id})`, { tag: ['@patterns', ...(id.startsWith('migrated-') ? ['@overlay'] : [])] }, () => {
    for (const state of entry.states) {
      test(`${state}: renders without layout defects`, async ({ page }) => {
        await open(page, id, state);
        const minTarget = isPhone(page) ? 40 : 0;
        await expectNoLayoutDefects(page, page.locator('[data-ui-lab-page]'), { minTarget });
        if (entry.overlay) {
          const sheet = page.getByRole('dialog');
          await expect(sheet).toBeVisible();
          const vw = page.viewportSize()!.width;
          // Wait for the Radix entrance transition before asserting geometry.
          await sheet.evaluate(async element => {await Promise.all(element.getAnimations().map(animation => animation.finished.catch(() => {})));});
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
        await page.screenshot({ path: test.info().outputPath(`${id}-${state}.png`) });
      });
    }
  });
}

test('gallery index lists every component and links to its first state', { tag: '@patterns' }, async ({ page }) => {
  await page.goto('/ui-lab/?fixture=gallery');
  const list = page.getByRole('list', { name: '组件列表' });
  await expect(list.getByRole('listitem')).toHaveCount(Object.keys(GALLERY_CATALOG).length);
  await expectNoLayoutDefects(page, page.locator('[data-ui-lab-page]'), { minTarget: isPhone(page) ? 40 : 0 });
});

test.describe('interactions', { tag: ['@patterns', '@overlay'] }, () => {
  test('new settings form built from shared patterns submits native form data', async ({ page }) => {
    await open(page, 'settings-form', 'normal');
    const sheet = page.getByRole('dialog', { name: '通知偏好设置' });
    await sheet.getByRole('switch', { name: '新的动态发布时提醒我' }).click();
    await sheet.getByLabel('开始').fill('22:30');
    await sheet.getByRole('combobox', { name: '摘要频率' }).click();
    await page.getByRole('option', { name: '每周一次' }).click();
    await sheet.getByLabel('备注').fill('周末可以晚一点提醒');
    await sheet.getByRole('button', { name: '保存设置' }).click();
    await expect(sheet).toBeHidden();
    const submitted = (await events(page)).find(e => e.action === 'settings-submit')!.args[0];
    expect(submitted).toMatchObject({ proactive: 'on', moment: 'on', promise: 'on', quietStart: '22:30', quietEnd: '07:30', digest: 'weekly', note: '周末可以晚一点提醒' });
  });

  test('edit sheet: required field blocks submit; busy sheet cannot be dismissed', async ({ page }) => {
    await open(page, 'edit-sheet', 'normal');
    const sheet = page.getByRole('dialog', { name: '编辑约定' });
    await sheet.getByRole('textbox', { name: '约定内容' }).fill('');
    await sheet.getByRole('button', { name: '保存' }).click();
    await expect(sheet).toBeVisible();
    expect((await events(page)).some(e => e.action === 'edit-submit')).toBe(false);
    await sheet.getByRole('textbox', { name: '约定内容' }).fill('周日去公园');
    await sheet.getByRole('button', { name: '保存' }).click();
    await expect(sheet).toBeHidden();
    expect((await events(page)).find(e => e.action === 'edit-submit')!.args[0]).toMatchObject({ title: '周日去公园', start: '10:00', end: '18:00' });

    await open(page, 'edit-sheet', 'busy');
    const busy = page.getByRole('dialog', { name: '编辑约定' });
    await expect(busy.getByRole('button', { name: '正在保存…' })).toBeDisabled();
    await expect(busy.getByRole('button', { name: '关闭' })).toHaveCount(0);
    await page.keyboard.press('Escape');
    await expect(busy).toBeVisible();
  });

  test('edit sheet error is announced and focused', async ({ page }) => {
    await open(page, 'edit-sheet', 'error');
    const alert = page.getByRole('dialog').getByRole('alert').first();
    await expect(alert).toContainText('草稿已保留');
    await expect(page.locator(':focus')).toContainText('草稿已保留');
  });

  test('tabs: keyboard navigation, disabled tab skipped, active tab visible', async ({ page }) => {
    await open(page, 'tabs', 'disabled');
    const list = page.getByRole('tablist', { name: '日程分区' });
    await list.getByRole('tab', { name: '当天安排' }).click();
    // Radix moves roving focus on a timer: wait for each move before the next key.
    await page.keyboard.press('ArrowRight');
    await expect(list.getByRole('tab', { name: '接下来几天' })).toBeFocused();
    await page.keyboard.press('ArrowRight');
    await expect(list.getByRole('tab', { name: '约定' })).toBeFocused();
    await expect(list.getByRole('tab', { name: '意向' })).toBeDisabled();
    await open(page, 'tabs', 'long');
    const active = page.getByRole('tab', { selected: true });
    await expect(active).toHaveText('历史记录与原始请求');
    await expect(active).toBeInViewport({ ratio: 1 });
  });

  test('migrated DetailDrawer keeps its test ids, footer and close behaviour', async ({ page }) => {
    await open(page, 'migrated-detail-drawer', 'normal');
    const sheet = page.getByTestId('migrated-drawer');
    await expect(sheet).toHaveAttribute('data-responsive-sheet', '');
    await expect(sheet.getByRole('button', { name: '完成' })).toBeInViewport({ ratio: 1 });
    await page.getByTestId('migrated-drawer-close').click();
    await expect(sheet).toBeHidden();
    await expect(page.getByRole('button', { name: '重新打开' })).toBeVisible();
  });

  test('migrated DrawerFrame keeps a single visible heading from its content', async ({ page }) => {
    await open(page, 'migrated-drawer-frame', 'normal');
    const sheet = page.getByRole('dialog', { name: '小雨' });
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole('heading', { name: '小雨' })).toHaveCount(2); // sr-only title + visible h2
    await expect(sheet.locator('[data-responsive-sheet-part="header"]')).toHaveClass(/sr-only/);
    await page.keyboard.press('Escape');
    await expect(sheet).toBeHidden();
  });

  test('data and task states expose retry/cancel actions that reach the fixture', async ({ page }) => {
    await open(page, 'record-list', 'error');
    await page.getByRole('button', { name: '重新读取约定' }).click();
    await open(page, 'task-status', 'failed');
    await page.getByRole('button', { name: '重试' }).click();
    await page.getByRole('button', { name: '查看技术详情' }).click();
    await expect(page.getByText('provider_timeout after 120s')).toBeVisible();
    const actions = (await events(page)).map(e => `${e.action}:${e.args[0]}`);
    expect(actions).toContain('retry:task');
    await open(page, 'task-status', 'running');
    await expect(page.getByRole('progressbar', { name: '处理进度' })).toBeVisible();
    await expect(page.locator('[data-task-state="running"]')).not.toHaveAttribute('data-feedback-kind', 'success');
  });
});

test('workspace dialog: phone opens on the title and busy cannot dismiss', {tag:['@patterns','@mobile']}, async({page})=>{
  await open(page,'workspace-dialog','page');
  if(isPhone(page))await expect(page.getByRole('heading',{name:'业务记录'})).toBeFocused();
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeHidden();
  await open(page,'workspace-dialog','busy');
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();
  await expect(page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true})).toHaveCount(0);
});

// Run only after the coordinated production build. Owns only its spawned fixture child.
import { createRequire } from 'node:module';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createServer } from 'node:net';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';
const require = createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const { chromium } = require('playwright-core');
const port = Number(process.env.WORLD_EXPANSION_PORT || 4451);
assert.ok(Number.isSafeInteger(port) && port >= 1024 && port <= 65535);
const base = `http://127.0.0.1:${port}`, fid = 'fluctlight-fixture-one', api = `/api/fluctlights/${fid}/world`;
const out = resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/world-expansion-states');
await mkdir(out, { recursive: true });
async function assertPortFree() {
  const probe = createServer();
  await new Promise((accept, reject) => {
    probe.once('error', e => reject(Error(`Refusing occupied fixture port ${port}: ${e.message}`)));
    probe.listen(port, '127.0.0.1', accept);
  });
  await new Promise(resolve => probe.close(resolve));
}
await assertPortFree();
const results = [];
const browser = await chromium.launch({ channel: 'chrome', headless: true });
async function startFixture(scenario) {
  // Refuse occupied ports; never kill or reuse an existing listener.
  await assertPortFree();
  const child = spawn(process.execPath, ['apps/web/test/fixtures/ui-quality-server.mjs'], {
    env: { ...process.env, FLUCTLIGHT_UI_FIXTURE_PORT: String(port), FLUCTLIGHT_UI_FIXTURE_WORLD: scenario, FLUCTLIGHT_UI_FIXTURE_DEMO: '' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let logs = '';
  try {
    await new Promise((accept, reject) => {
      const timer = setTimeout(() => reject(Error('Fixture startup timed out: ' + logs)), 10000);
      child.once('error', e => { clearTimeout(timer); reject(e); });
      child.once('exit', code => { clearTimeout(timer); reject(Error('Fixture exited ' + code + ': ' + logs)); });
      child.stderr.on('data', data => { logs += data; });
      child.stdout.on('data', data => {
        logs += data;
        if (logs.includes(`Isolated UI fixture: ${base}`)) { clearTimeout(timer); accept(); }
      });
    });
    return child;
  } catch (e) { if (child.exitCode === null) child.kill('SIGTERM'); throw e; }
}
async function run(kind, scenario, size) {
  const record = { kind, scenario, size, evidence: 'production components / controlled HTTP fixture; no Core, DB, worker or model proof', checks: [], requests: [], errors: [], screenshots: [] };
  let child, context, page;
  const check = (name, value) => { assert.ok(value, name); record.checks.push(name); };
  try {
    child = await startFixture(scenario);
    context = await browser.newContext({ viewport: { width: size === 'mobile' ? 390 : 1348, height: size === 'mobile' ? 844 : 925 }, deviceScaleFactor: 1, locale: 'zh-CN', timezoneId: 'Asia/Shanghai', reducedMotion: 'reduce' });
    page = await context.newPage(); page.setDefaultTimeout(12000);
    page.on('pageerror', e => record.errors.push(e.message));
    page.on('request', r => { if (new URL(r.url()).pathname.startsWith('/api/')) record.requests.push({ path: new URL(r.url()).pathname, method: r.method(), body: r.postDataJSON() }); });
    const dialog = () => page.getByRole('dialog').last();
    const writes = () => record.requests.filter(r => !['GET', 'HEAD'].includes(r.method));
    const get = async path => { const r = await page.request.get(base + path); assert.equal(r.status(), 200, path); return r.json(); };
    const shot = async state => {
      await page.evaluate(async () => { await document.fonts.ready; });
      const path = resolve(out, `${kind}-${scenario}-${size}-${state}.png`);
      await page.screenshot({ path, animations: 'disabled' }); record.screenshots.push(path);
      check(state + ' document fits', await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      const box = await dialog().boundingBox();
      if (box) check(state + ' dialog fits', box.x >= 0 && box.y >= 0 && box.x + box.width <= page.viewportSize().width + 1 && box.y + box.height <= page.viewportSize().height + 1);
    };
    await page.goto(`${base}/#/c/${fid}/world`, { waitUntil: 'networkidle' });
    await page.getByRole('button', { name: '进入世界 温和魔法世界', exact: true }).click();
    if (kind === 'failed-run') {
      await page.getByRole('tab', { name: '世界事件', exact: true }).click();
      await page.locator('.r-event-row').getByRole('button', { name: '详情', exact: true }).click();
      await dialog().getByText('执行失败', { exact: true }).waitFor(); await shot('failed');
      const before = (await get(api + `/templates/${fid}-template/runs`)).items[0];
      await dialog().getByRole('button', { name: '恢复这次失败执行', exact: true }).click();
      await dialog().getByRole('textbox', { name: '世界事件操作说明', exact: true }).fill('恢复同一失败执行，保留原窗口');
      await dialog().getByRole('button', { name: '恢复这次失败执行', exact: true }).click();
      if (scenario !== 'expansion-normal') {
        const recovery = scenario.endsWith('unknown') ? '核对或重试原提交' : '只读恢复页面结果';
        await dialog().getByRole('button', { name: recovery, exact: true }).waitFor(); await shot('recovery');
        await dialog().getByRole('button', { name: recovery, exact: true }).click();
      }
      await dialog().getByRole('button', { name: '刷新事件与执行', exact: true }).waitFor();
      await dialog().getByRole('button', { name: '查看实际生活事件', exact: true }).waitFor(); await shot('applied');
      const rows = (await get(api + `/templates/${fid}-template/runs`)).items;
      check('same run follows actual fixture result', rows.length === 1 && rows[0].id === before.id && rows[0].status === 'applied');
      const sent = writes(); check('no trigger creates a replacement run', sent.every(r => r.path.endsWith(`/${before.id}/retry`)));
      check('template and run CAS retained', sent[0].body.expectedRevision === 6 && sent[0].body.expectedRunRevision === 4);
      if (scenario.endsWith('unknown')) { assert.deepEqual(sent[1], sent[0]); check('original body/key replay', sent.length === 2); }
      else check('one mutation including readback recovery', sent.length === 1);
    } else if (kind === 'effective-baseline') {
      await page.locator('.r-place-card').filter({ has: page.getByRole('heading', { name: '迟山书店', exact: true }) }).getByRole('button', { name: '修订记录', exact: true }).click();
      await dialog().getByText('已应用定义中的书店：木窗、长桌与旧书架。', { exact: true }).waitFor();
      await shot('effective-definition');
      const before = await get(api + `/places/${fid}-shop/defaults?worldId=fixture-world&source=recorded`);
      check('effective publication is not a fabricated saved snapshot', before.default_revision === 0 && before.baseline.source_kind === 'published_definition');
      await dialog().getByRole('button', { name: '恢复默认字段', exact: true }).click();
      await dialog().getByRole('textbox', { name: '地点基准操作说明', exact: true }).fill('明确恢复已验证来源的有效基准');
      await dialog().getByRole('button', { name: '确认恢复默认字段', exact: true }).click();
      await dialog().getByRole('button', { name: '已记录的恢复基准', exact: true }).waitFor();
      const after = await get(api + `/places/${fid}-shop/defaults?worldId=fixture-world&source=recorded`);
      check('explicit restore records effective baseline and advances place', after.default_revision === 1 && after.place_revision === 2 && after.current.description === before.baseline.definition.description);
      check('zero baseline revision is preserved in original command', writes().length === 1 && writes()[0].body.expectedDefaultRevision === 0);
      await shot('effective-restored');
    } else {
      const before = (await get(api + '/catalog/fixture-world')).places.find(p => p.id === fid + '-shop');
      await page.locator('.r-place-card').filter({ has: page.getByRole('heading', { name: '迟山书店', exact: true }) }).getByRole('button', { name: '修订记录', exact: true }).click();
      await dialog().getByRole('button', { name: '预览当前已保存字段', exact: true }).click();
      await dialog().getByRole('button', { name: '明确保存此恢复基准', exact: true }).waitFor(); await shot('saved-preview');
      await dialog().getByRole('button', { name: '预览已应用定义', exact: true }).click();
      await dialog().getByText('已应用定义中的书店：木窗、长桌与旧书架。', { exact: true }).waitFor(); await shot('definition-preview');
      check('preview sends no mutations', writes().length === 0);
      await dialog().getByRole('button', { name: '明确保存此恢复基准', exact: true }).click();
      await dialog().getByRole('textbox', { name: '地点基准操作说明', exact: true }).fill('冻结真实已应用定义，保持当前地点');
      await dialog().getByRole('button', { name: '确认保存恢复基准', exact: true }).click();
      if (scenario !== 'expansion-normal') {
        const recovery = scenario.endsWith('unknown') ? '核对或重试原提交' : '只读恢复页面结果';
        await dialog().getByRole('button', { name: recovery, exact: true }).waitFor(); await shot('recovery');
        await dialog().getByRole('button', { name: recovery, exact: true }).click();
      }
      await dialog().getByRole('button', { name: '已记录的恢复基准', exact: true }).waitFor();
      const saved = (await get(api + '/catalog/fixture-world')).places.find(p => p.id === before.id);
      assert.deepEqual(saved, before); check('capture preserves current fields', true);
      const baseline = await get(api + `/places/${before.id}/defaults?worldId=fixture-world&source=recorded`);
      check('recorded baseline retains exact published source', baseline.baseline.source_kind === 'published_definition' && baseline.baseline.source_version === 'v1' && baseline.default_revision === 2);
      if (scenario.endsWith('unknown')) { assert.deepEqual(writes()[1], writes()[0]); check('original capture body/key replay', writes().length === 2); }
      else check('capture only once', writes().length === 1);
      await dialog().getByRole('button', { name: '已记录的恢复基准', exact: true }).click();
      await dialog().getByRole('button', { name: '恢复默认字段', exact: true }).click();
      await dialog().getByRole('textbox', { name: '地点基准操作说明', exact: true }).fill('明确恢复已核对定义');
      await dialog().getByRole('button', { name: '确认恢复默认字段', exact: true }).click();
      if (scenario.endsWith('readback-failure')) await dialog().getByRole('button', { name: '只读恢复页面结果', exact: true }).click();
      await dialog().getByRole('button', { name: '已记录的恢复基准', exact: true }).waitFor(); await shot('restored');
      await page.keyboard.press('Escape'); await page.reload({ waitUntil: 'networkidle' });
      await page.getByRole('button', { name: '进入世界 温和魔法世界', exact: true }).click();
      const after = (await get(api + '/catalog/fixture-world')).places.find(p => p.id === before.id);
      check('explicit restore persists across refresh within fixture', after.desc === '已应用定义中的书店：木窗、长桌与旧书架。' && after.revision === before.revision + 1);
      assert.deepEqual(after.connection_place_ids, before.connection_place_ids);
    }
    check('no browser exceptions', record.errors.length === 0); record.status = 'passed';
  } catch (e) {
    record.status = 'failed'; record.error = e.stack;
    if (page) { record.text = await page.locator('body').innerText().catch(() => ''); await page.screenshot({ path: resolve(out, `${kind}-${scenario}-${size}-failure.png`) }).catch(() => {}); }
  } finally {
    await context?.close();
    if (child && child.exitCode === null) { const exited = once(child, 'exit'); child.kill('SIGTERM'); await exited; }
    results.push(record); await writeFile(resolve(out, 'report.json'), JSON.stringify(results, null, 2) + '\n');
    console.log(kind, scenario, size, record.status);
  }
}
try {
  for (const size of ['desktop', 'mobile']) {
    for (const scenario of ['expansion-normal', 'expansion-unknown', 'expansion-readback-failure']) {
      await run('failed-run', scenario, size); await run('definition-baseline', scenario, size);
    }
    await run('effective-baseline', 'expansion-effective', size);
  }
} finally { await browser.close(); }
if (results.some(r => r.status !== 'passed')) process.exitCode = 1;

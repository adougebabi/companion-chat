import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {mountReact, installDOM, act} from './helpers/react-dom.mjs';
import {approvedRoute} from '../src/approved-ui/routing.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false, appType: 'custom', logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => { await vite.close(); env.restore(); });

async function mountSettings(section) {
  const {default: Management} = await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
  const state = proxy({session: {actorId: 'settings-route-test'}, selectedId: '', characters: [],
    operations: [], managementTab: '配置中心', managementSection: section, runtimeCatalog: {}});
  let writes = 0;
  const workspace = {state, subscribe: fn => subscribe(state, fn), getSnapshot: () => snapshot(state),
    notify() {}, navigatePath() {}, services: {
      configurationClient: {
        read: async () => ({effective_values: {'plugins.domains': {image: false, moment: false}},
          versions: {}, value_sources: {}, endpoints: [], bindings: []}),
        catalog: async () => ({agents: []}), models: async () => ({models: []}),
        save: async () => { writes++; throw Error('禁止写入'); },
      },
      diagnosticsClient: {page: async () => ({items: [], next_cursor: ''})},
    }};
  const view = await mountReact(vite, Management, {workspace});
  return {view, state, writes: () => writes};
}

const visibleSection = (host, section) => [...host.querySelectorAll('.settings-group')]
  .find(node => node.dataset.section === section && !node.hidden);

test('直接打开旧能力链接后呈现能力插件卡片和开关', async () => {
  const route = approvedRoute('#/settings/capabilities');
  const {view, writes} = await mountSettings(route.query.get('section'));
  try {
    const panel = visibleSection(view.host, '能力插件');
    assert.ok(panel, '插件分区必须可见');
    assert.match(panel.textContent, /启用图片能力/);
    assert.match(panel.textContent, /启用动态能力/);
    const controls=[...panel.querySelectorAll('[role=checkbox]')];
    assert.equal(controls.length,2);
    assert.ok(controls.every(node=>node.getAttribute('aria-checked')==='false'));
    assert.ok(controls.every(node=>node.id&&panel.querySelector('label[for="'+node.id+'"]')));
    assert.equal(view.host.querySelector('[data-testid=config-save]').disabled, true);
    assert.equal(writes(), 0);
    await view.click(controls[0]);
    assert.equal(controls[0].getAttribute('aria-checked'),'true');
    assert.equal(view.host.querySelector('[data-testid=config-save]').disabled,false);
    assert.equal(writes(),0,'editing a shared checkbox does not save automatically');
  } finally { await view.dispose(); }
});

test('未知旧子路径与退役分区恢复值都展示默认模型服务面板', async () => {
  for (const section of [approvedRoute('#/settings/owner').query.get('section'), '能力与权限', 'private-key']) {
    const {view, writes} = await mountSettings(section);
    try {
      const panel = visibleSection(view.host, '服务地址');
      assert.ok(panel, section);
      assert.match(panel.textContent, /尚未配置模型服务/);
      const tab = [...view.host.querySelectorAll('[role=tab]')].find(node => node.textContent.startsWith('模型服务'));
      assert.equal(tab.getAttribute('aria-selected'), 'true');
      assert.equal(writes(), 0);
    } finally { await view.dispose(); }
  }
});

test('已挂载设置页切换到无效分区后仍有选中的模型服务 Tab 与内容', async () => {
  const {view, state, writes} = await mountSettings('能力插件');
  try {
    assert.ok(visibleSection(view.host, '能力插件'));
    await act(async () => { state.managementSection = 'private-key'; });
    await view.flush();
    assert.ok(visibleSection(view.host, '服务地址'));
    const tab = [...view.host.querySelectorAll('[role=tab]')].find(node => node.textContent.startsWith('模型服务'));
    assert.equal(tab.getAttribute('aria-selected'), 'true');
    assert.doesNotMatch(view.host.textContent, /private-key|能力与权限/);
    assert.equal(writes(), 0);
  } finally { await view.dispose(); }
});

import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {mountReact} from './helpers/react-dom.mjs';

test('能力插件摘要隐藏技术字段，每个诊断入口下载对应的完整原始记录', async t => {
  const vite = await createServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    envFile: false,
    appType: 'custom',
    logLevel: 'silent',
    server: {middlewareMode: true},
  });
  t.after(() => vite.close());
  const {default: SettingsSections} = await vite.ssrLoadModule('/src/react-app/shared/SettingsSections.jsx');
  const plugins = [
    {id: 'image', version: '1.42.0', enabled: true, health: 'not_checked', required_dependencies: ['comfyui'],
      revision: 'private-revision', source_ref: 'private-source', idempotency_key: 'private-key', metrics: {count: 42}},
    {id: 'moment', version: '2.81.0', enabled: false, health: 'not_checked', optional_dependencies: ['image']},
    {id: 'private-plugin-id', enabled: false, health: 'not_checked'},
  ];
  const group = {title: '能力插件', fields: []};
  const view = await mountReact(vite, SettingsSections, {props: {group, view: {
    catalogReady: {value: true},
    domainPlugins: {value: plugins},
    sectionId: () => 'capabilities',
    mediaHealthScope: () => ({owner: 'fixture', version: 1}),
    momentHealthScope: () => ({owner: 'fixture', version: 1}),
    test: () => assert.fail('打开列表和下载诊断不应调用健康检查'),
  }}});
  try {
    const blobs = new Map();
    t.mock.method(URL, 'createObjectURL', blob => {
      const url = `blob:fixture/${blobs.size}`;
      blobs.set(url, blob);
      return url;
    });
    const downloads = [];
    t.mock.method(window.HTMLAnchorElement.prototype, 'click', function () {
      downloads.push({filename: this.download, blob: blobs.get(this.href)});
    });
    const buttons = (root, label) => [...root.querySelectorAll('button')].filter(node => node.textContent.trim() === label);
    const title = '原始插件记录（诊断）';
    const assertNoTechnicalText = () => assert.doesNotMatch(view.host.textContent,
      /\b(?:id|image|moment|version|revision|source_ref|idempotency_key|metrics|not_checked|required_dependencies|optional_dependencies|comfyui)\b|版本|1\.42\.0|2\.81\.0|private-(?:plugin-id|revision|source|key)|尚未记录/);
    const summaries = [...view.host.querySelectorAll('p.muted')];
    assert.deepEqual(summaries.map(node => node.textContent), [
      '图片 · 已启用 · 健康状态未检测 · 生图需要 ComfyUI',
      '动态 · 已停用 · 健康状态未检测 · 图片为可选依赖',
      '插件 · 已停用 · 健康状态未检测',
    ]);
    assertNoTechnicalText();
    assert.equal(buttons(view.host, title).length, plugins.length);
    for (const summary of summaries) {
      const row = summary.parentElement;
      const [diagnostic] = buttons(row, title);
      assert.ok(diagnostic);
      await view.click(diagnostic);
      assertNoTechnicalText();
      const [download] = buttons(row, '下载原始记录');
      assert.ok(download);
      await view.click(download);
    }
    assert.deepEqual(downloads.map(({filename}) => filename), plugins.map(() => 'fluctlight-record.json'));
    assert.deepEqual(await Promise.all(downloads.map(async ({blob}) => JSON.parse(await blob.text()))), plugins);
  } finally {
    await view.dispose();
  }
});

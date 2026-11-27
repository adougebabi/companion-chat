import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {mountReact} from './helpers/react-dom.mjs';

test('能力插件使用首个可读的自带名称，保留内置中文名和诊断下载', async t => {
  const vite = await createServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    envFile: false,
    appType: 'custom',
    logLevel: 'silent',
    server: {middlewareMode: true},
  });
  t.after(() => vite.close());

  const {default: SettingsSections} = await vite.ssrLoadModule('/src/react-app/shared/SettingsSections.jsx');
  const records = [
    ['天气预报', {display_name: '  天气预报  ', title: '次选标题', name: '次选名称', manifest: {display_name: '清单首选'}}],
    ['日程提醒', {display_name: 'req_42', title: '日程提醒', name: '次选名称'}],
    ['Music Box', {display_name: 'music_box', title: ' ', name: 'Music Box', manifest: {display_name: '清单首选'}}],
    ['出行助手', {display_name: null, title: 17, name: 'route_helper', manifest: {display_name: '出行助手', title: '清单次选', name: '清单次选名称'}}],
    ['阅读清单', {manifest: {display_name: 'actor_77', title: '阅读清单', name: '清单次选名称'}}],
    ['单词本', {manifest: {display_name: 'VOCAB_PLUGIN', title: {}, name: '单词本'}}],
    ['插件', {display_name: ' ', name: 'fallback_plugin', manifest: {title: 'mem_24'}}],
    ['插件', {}],
    ['图片', {id: 'image', display_name: '其他图片名称'}],
    ['动态', {id: 'moment', manifest: {display_name: '其他动态名称'}}],
  ];
  const plugins = records.map(([, names], index) => ({
    id: `private-plugin-${index}`,
    enabled: true,
    health: 'not_checked',
    version: '9.8.7',
    revision: 'private-revision',
    source_ref: 'private-source',
    idempotency_key: 'private-key',
    metrics: {count: 42},
    ...names,
  }));
  const mounted = await mountReact(vite, SettingsSections, {props: {
    group: {title: '能力插件', fields: []},
    view: {
      catalogReady: {value: true},
      domainPlugins: {value: plugins},
      sectionId: () => 'capabilities',
      mediaHealthScope: () => ({owner: 'fixture', version: 1}),
      momentHealthScope: () => ({owner: 'fixture', version: 1}),
      test: () => assert.fail('显示插件名称和展开诊断不应发起健康检查'),
    },
  }});
  try {
    const summaries = [...mounted.host.querySelectorAll('p.muted')];
    assert.deepEqual(summaries.map(row => row.textContent), records.map(([name]) => `${name} · 已启用 · 健康状态未检测`));
    const hiddenText = /\b(?:id|version|revision|source_ref|idempotency_key|metrics|display_name|manifest|not_checked)\b|版本|9\.8\.7|private-(?:plugin-\d+|revision|source|key)|req_42|music_box|route_helper|actor_77|VOCAB_PLUGIN|fallback_plugin|mem_24|尚未记录/;
    assert.doesNotMatch(mounted.host.textContent, hiddenText);

    for (const summary of summaries) {
      const row = summary.parentElement;
      const button = label => [...row.querySelectorAll('button')].find(node => node.textContent.trim() === label);
      const diagnostic = button('原始插件记录（诊断）');
      assert.ok(diagnostic);
      await mounted.click(diagnostic);
      assert.ok(button('下载原始记录'));
      assert.doesNotMatch(row.textContent, hiddenText);
    }
  } finally {
    await mounted.dispose();
  }
});

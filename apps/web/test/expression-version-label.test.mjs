import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';

test('聊天回复正文和下拉显示中文表达设定，保存沿用原值', async () => {
  const env = installDOM();
  const vite = await createServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    envFile: false, appType: 'custom', logLevel: 'silent',
    server: {middlewareMode: true, ws: false},
  });
  const settingKey = 'conversation.reply';
  const choices = [
    ['自然交流 · 当前表达设定', 'fact-only-reply.v8'],
    ['自然交流 · 兼容上一套设定', 'fact-only-reply.v7'],
    ['自然交流 · 兼容较早设定', 'fact-only-reply.v6'],
    ['自然交流 · 兼容早期设定', 'fact-only-reply.v5'],
    ['自然交流 · 兼容最早设定', 'fact-only-reply.v4'],
  ];
  const hiddenCopy = /\bv\d+\b|协议|fact-only-reply|表达版本|expression_version|\b(?:id|revision|source_ref|idempotency|metrics)\b|尚未记录/i;
  const state = proxy({session: {actorId: 'expression-label-test'}, selectedId: '', characters: [],
    operations: [], managementTab: '配置中心', managementSection: '聊天回复', runtimeCatalog: {}});
  const submissions = [];
  const workspace = {
    state, subscribe: fn => subscribe(state, fn), getSnapshot: () => snapshot(state),
    notify() {}, navigatePath() {}, services: {
      configurationClient: {
        read: async () => ({effective_values: {[settingKey]: {mode: 'fact_only', expression_version: choices[0][1]}},
          versions: {}, value_sources: {}, endpoints: [], bindings: []}),
        catalog: async () => ({agents: []}),
        save: async body => { submissions.push(body.values); },
        command: async () => ({status: 'applied'}),
      },
      diagnosticsClient: {page: async () => ({items: [], next_cursor: ''})},
    },
  };

  try {
    const {default: Management} = await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
    const view = await mountReact(vite, Management, {workspace});
    try {
      const panel = view.host.querySelector('[data-section="聊天回复"]:not([hidden])');
      assert.ok(panel);
      const trigger = panel.querySelector('[role="combobox"][aria-label="表达设定"]');
      assert.ok(trigger);
      assert.equal(trigger.textContent, choices[0][0]);
      assert.doesNotMatch(panel.textContent, hiddenCopy);
      const save = view.host.querySelector('[data-testid="config-save"]');
      assert.equal(save.disabled, true);

      for (const [label, value] of [...choices.slice(1), choices[0]]) {
        await act(async () => trigger.dispatchEvent(new window.KeyboardEvent('keydown', {key: 'ArrowDown', bubbles: true})));
        await view.flush();
        const options = [...document.querySelectorAll('[role="listbox"] [role="option"]')];
        assert.deepEqual(options.map(option => option.textContent), choices.map(([name]) => name));
        assert.doesNotMatch(options.map(option => option.textContent).join(' '), hiddenCopy);

        await view.click(options.find(option => option.textContent === label));
        assert.equal(trigger.textContent, label);
        assert.doesNotMatch(panel.textContent, hiddenCopy);
        assert.equal(save.disabled, false);
        const count = submissions.length;
        await view.click(save);
        assert.equal(submissions.length, count + 1);
        assert.deepEqual(submissions.at(-1), {[settingKey]: {mode: 'fact_only', expression_version: value}});
        assert.equal(save.disabled, true);
      }
    } finally {
      await view.dispose();
    }
  } finally {
    await vite.close();
    env.restore();
  }
});

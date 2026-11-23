import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

const environment = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  configFile: false,
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  esbuild: {jsx: 'automatic'},
  resolve: {alias: {'@': fileURLToPath(new URL('../src', import.meta.url))}},
  server: {middlewareMode: true, ws: false},
});
test.after(async () => { await vite.close(); environment.restore(); });

const internal = {
  id: 'private-record-id',
  revision: 'private-record-revision',
  source_ref: 'private-source-ref',
  idempotencyKey: 'private-request-key',
  metrics: {trust: 'private-metric-value'},
};
const pending = {...internal, reason: '保留原操作理由', nested: {flag: false, count: 0, items: ['原文 <&>']}};
const snapshot = {...internal, text: '要核对的原消息 <&>', source_valid: false, created_at: '2026-11-23T08:00:00Z'};
const latest = {
  ...internal, excerpt: '要核对的候选摘录', status: 'accepted',
  identity: {name: '纪岚', ...internal}, nested: [{...internal}],
};
const help = '这项操作的完整内容可在诊断记录中下载。';
const conflictStatus = '资料版本已变化，草稿已保留，请重新读取后核对。';
const button = (host, label) => [...host.querySelectorAll('button')].find(node => node.textContent.trim() === label);

function assertNoRawFields(host) {
  assert.doesNotMatch(host.textContent.replace(conflictStatus, ''), /\bid\b|revision|版本|source[_ ]ref|idempotency|metrics|private-|尚未记录/i);
  assert.equal(host.querySelector('pre, code'), null);
}

async function assertDownloads(view, host, records) {
  const downloads = [];
  const create = URL.createObjectURL, revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype, click = anchor.click;
  let blob;
  URL.createObjectURL = value => { blob = value; return 'blob:command-record'; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { downloads.push({filename: this.download, blob}); };
  try {
    for (const [title] of records) {
      const disclosure = button(host, title);
      assert.ok(disclosure, title);
      await view.click(disclosure);
      assertNoRawFields(host);
      const download = button(host, '下载原始记录');
      assert.ok(download, title);
      await view.click(download);
      await view.click(disclosure);
    }
    assert.equal(downloads.length, records.length);
    for (const [index, {filename, blob: downloaded}] of downloads.entries()) {
      assert.equal(filename, 'fluctlight-record.json');
      assert.equal(downloaded.type, 'application/json');
      assert.deepEqual(JSON.parse(await downloaded.text()), records[index][1], records[index][0]);
    }
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    anchor.click = click;
  }
}

for (const [file, records, reviewTitle] of [
  ['WorkspaceCommandDialog.jsx', [['原始提交记录（诊断）', pending], ['原始核对记录（诊断）', latest]], '核对最新资料'],
  ['CommandDialog.tsx', [['原始提交记录（诊断）', pending], ['操作打开时的原始记录（诊断）', snapshot], ['原始核对记录（诊断）', latest]], '核对最新对象'],
]) {
  test(`${file} 回退隐藏内部字段并逐份下载完整记录，仍可确认最新基线`, async () => {
    const {default: Dialog} = await vite.ssrLoadModule(`/src/react-app/shared/${file}`);
    const action = {
      title: '核对这项操作',
      identity: file,
      scope: file,
      initial: {},
      fields: () => null,
      snapshot,
      build: () => pending,
      execute: async () => { throw Object.assign(Error('资料已更新，请先核对。'), {status: 409, code: 'revision_conflict'}); },
      readLatest: async () => latest,
    };
    const view = await mountReact(vite, Dialog, {
      workspace: {state: {session: {actorId: 'test-owner'}}},
      props: {action, onClose() {}},
    });
    try {
      const host = document.querySelector('[role=dialog]');
      const submit = button(host, '确认提交');
      assert.ok(submit);
      await view.click(submit);
      const sync = button(host, '同步资料，保留输入');
      assert.ok(sync);
      await view.click(sync);
      for (const label of [action.title, conflictStatus, reviewTitle, help]) {
        assert.ok(host.textContent.includes(label), label);
      }
      if (file === 'CommandDialog.tsx') {
        for (const label of [pending.reason, snapshot.text, '来源已变化。', latest.excerpt, latest.identity.name, '状态：已接受']) {
          assert.ok(host.textContent.includes(label), label);
        }
      }
      assertNoRawFields(host);
      await assertDownloads(view, host, records);
      const confirm = button(host, '已核对，采用最新基线');
      assert.ok(confirm);
      await view.click(confirm);
      assert.match(host.textContent, /最新基线已明确确认，当前输入仍保留/);
    } finally {
      await view.dispose();
    }
  });
}

test('未迁移命令的空对象提交仍可下载且无空值占位', async () => {
  const {default: Dialog} = await vite.ssrLoadModule('/src/react-app/shared/CommandDialog.tsx');
  const view = await mountReact(vite, Dialog, {
    workspace: {state: {session: {actorId: 'test-owner'}}},
    props: {
      action: {
        identity: 'empty-command', scope: 'empty-command', title: '提取候选',
        initial: {}, fields: () => null, snapshot: null, build: () => ({}),
        execute: async () => { throw Error('原提交待核对。'); },
      },
      onClose() {},
    },
  });
  try {
    const host = document.querySelector('[role=dialog]');
    assert.ok(!host.textContent.includes(help));
    await view.click(button(host, '确认提交'));
    assert.ok(host.textContent.includes(help));
    assert.ok(button(host, '核对或重试原提交'));
    assertNoRawFields(host);
    await assertDownloads(view, host, [['原始提交记录（诊断）', {}]]);
  } finally {
    await view.dispose();
  }
});

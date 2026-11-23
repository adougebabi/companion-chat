import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => {
  await vite.close();
  env.restore();
});
const {default: Inbox} = await vite.ssrLoadModule('/src/react-app/pages/InboxPage.jsx');

const notice = {
  id: 'notice_private',
  fluctlight_id: 'fluctlight_private',
  title: '新回复',
  body: '通知保存的原始正文。',
  link: '#/c/fluctlight_private/chat?conversation=conversation_private&message=message_private',
  quiet: false,
  read_at: null,
  created_at: '2026-11-23T03:12:00Z',
  source_kind: 'conversation.reply.published',
  source_id: 'source_private',
  source_event_id: 'event_private',
  source_occurred_at: '2026-11-23T02:25:00Z',
  revision: 937,
  source_ref: 'reference_private',
  idempotency_key: 'command_private',
  metrics: {attempts: 781},
};
const receiptTime = '2026-11-23T04:21:00Z';
const publicBody = '我刚在河岸看到了第一场雪。';

function workspace(row, markNotificationRead = async () => ({read_at: receiptTime})) {
  const state = {
    session: {actorId: 'inbox-raw-fields-owner'},
    characters: [{id: notice.fluctlight_id, name: '青岚'}],
  };
  return {
    state,
    current: {value: {}},
    getSnapshot: () => state,
    subscribe: () => () => {},
    client: {
      messages: async () => ({
        conversation: {id: 'conversation_private'},
        messages: [{
          id: 'message_private', conversationId: 'conversation_private',
          kind: 'assistant', authorActorId: notice.fluctlight_id, text: publicBody,
        }],
      }),
    },
    markNotificationRead,
    services: {operationsClient: {
      notifications: async () => ({items: [row], next_cursor: ''}),
      pending: async () => ({groups: []}),
    }},
  };
}

function assertBusinessView(root) {
  assert.doesNotMatch(root.textContent, /\b(?:id|fluctlight_id|source_kind|source_id|source_event_id|source_ref|read_at|link|revision|idempotency_key|metrics|attempts|937|781)\b|版本|尚未记录|未记录|时间未知/);
  assert.doesNotMatch(root.textContent, /\b\w+_private\b|conversation\.reply\.published|#\/c\/|2026-11-23T\d{2}:\d{2}:\d{2}Z/);
  assert.equal(root.querySelector('.record-facts, pre, code'), null);
}

const button = (root, label) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === label);

test('通知摘要显示正文、发生时间和已读状态，并仅下载完整原始通知', async (t) => {
  const read = Promise.withResolvers();
  const view = await mountReact(vite, Inbox, {workspace: workspace(notice, () => read.promise)});
  try {
    const card = view.host.querySelector('.replica-notice');
    assert.equal(card.querySelector('h3').textContent, '青岚 · 新回复');
    assert.equal(card.querySelector('.replica-notice-summary').textContent, publicBody);
    assert.equal(card.querySelector('time').textContent, '2026-11-23 10:25 (北京时间)');
    assertBusinessView(card);

    await view.click(card.querySelector('.replica-notice-main'));
    const dialog = document.querySelector('[role=dialog]');
    assert.equal(dialog.querySelector('.replica-inbox-body').textContent, publicBody);
    assert.match(dialog.textContent, /发生时间：2026-11-23 10:25 \(北京时间\) · 未读/);
    read.resolve({id: notice.id, read_at: receiptTime});
    await view.flush();
    assert.match(dialog.textContent, /发生时间：2026-11-23 10:25 \(北京时间\) · 已读/);
    assert.equal(card.querySelector('.replica-notice-actions').textContent, '已读');

    const diagnostic = button(dialog, '原始通知记录（诊断）');
    assert.ok(diagnostic);
    await view.click(diagnostic);
    assertBusinessView(dialog);
    assert.doesNotMatch(dialog.textContent, /通知保存的原始正文/);

    const blobs = [], downloads = [];
    t.mock.method(URL, 'createObjectURL', (blob) => {
      blobs.push(blob);
      return 'blob:inbox-record';
    });
    t.mock.method(URL, 'revokeObjectURL', () => {});
    const clickAnchor = window.HTMLAnchorElement.prototype.click;
    t.mock.method(window.HTMLAnchorElement.prototype, 'click', function () {
      if (this.download) downloads.push({filename: this.download, href: this.href});
      else clickAnchor.call(this);
    });
    await view.click(button(dialog, '下载原始记录'));
    assert.deepEqual(downloads, [{filename: 'fluctlight-record.json', href: 'blob:inbox-record'}]);
    assert.equal(blobs[0].type, 'application/json');
    assert.deepEqual(JSON.parse(await blobs[0].text()), notice);

    const source = [...dialog.querySelectorAll('a')].find(node => node.textContent.trim() === '查看原始来源');
    const href = notice.link + '&returnTo=%23%2Finbox';
    assert.equal(source.getAttribute('href'), href);
    await view.click(source);
    assert.equal(location.hash, href);
  } finally {
    await view.dispose();
  }
});

test('通知缺少正文、发生时间或可访问来源时省略相应区域', async () => {
  const row = {
    id: notice.id, title: '提醒', body: '', link: '//invalid.example/notice',
    read_at: receiptTime, source_occurred_at: '', occurred_at: null, created_at: '',
  };
  const view = await mountReact(vite, Inbox, {workspace: workspace(row)});
  try {
    const card = view.host.querySelector('.replica-notice');
    assert.equal(card.querySelector('time'), null);
    assertBusinessView(card);
    await view.click(card.querySelector('.replica-notice-main'));
    const dialog = document.querySelector('[role=dialog]');
    assert.equal(dialog.querySelector('.replica-inbox-body, time, a'), null);
    assert.equal(button(dialog, '查看原始来源'), undefined);
    assert.equal(dialog.querySelector('.replica-inbox-caption').textContent.trim(), '已读');
    assert.ok(button(dialog, '原始通知记录（诊断）'));
    assertBusinessView(dialog);
  } finally {
    await view.dispose();
  }
});

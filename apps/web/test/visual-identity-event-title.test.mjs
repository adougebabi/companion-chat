import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {mediaFixture} from './fixtures/media-fixture.mjs';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

const dom = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => { await vite.close(); dom.restore(); });

const failureCode = 'visual_identity_candidate_media_failed';
const failure = {
  id: 'private-event-id',
  stage: 'failed',
  status: 'failed',
  summary: failureCode,
  occurred_at: '2026-11-20T01:29:00Z',
  revision: 37,
  source_ref: 'private-source-ref',
  idempotency_key: 'private-request-key',
  metrics: {correlation_id: 'private-correlation-id'},
};
const diagnosticTitle = '原始事件记录（诊断）';
const button = (root, label) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === label);

async function mountTimeline(t, events) {
  const {createWorkspace} = await vite.ssrLoadModule('/src/state/workspace.mjs');
  const store = await vite.ssrLoadModule('/src/v3/proto/store.js');
  const {default: MediaPage} = await vite.ssrLoadModule('/src/react-app/pages/MediaPage.tsx');
  const api = mediaFixture();
  api.state().timeline = events;
  const workspace = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: {actorId: 'media-owner'},
    characters: {A: {id: 'A', name: '林青', status: 'active'}},
    ui: {charId: 'A'},
  });
  workspace.client.fetcher = async (input, init = {}) => {
    const method = init.method || 'GET';
    assert.equal(method, 'GET');
    const url = new URL(input, 'http://media.test');
    const path = url.pathname;
    let response;
    const handled = await api.handler({
      url, path, method, body: {},
      json: (_res, row, status = 200) => { response = Response.json(row, {status}); },
    });
    assert.ok(handled, path);
    return response;
  };
  const view = await mountReact(vite, MediaPage, {
    workspace,
    props: {initialTab: 'identity', route: {path: '/c/A/media/identity', fid: 'A', query: new URLSearchParams()}},
  });
  t.after(async () => { await view.dispose(); workspace.dispose(); });
  return view;
}

function assertCleanEvent(entry) {
  assert.doesNotMatch(entry.textContent, /\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b|\b(?:id|revision|metrics)\b|private-|版本|尚未记录/i);
  assert.equal(entry.querySelector('pre, code'), null);
  assert.ok(button(entry, diagnosticTitle));
}

test('视觉时间线翻译已知码，未知码显示通用标题，保留业务摘要、评审和图片入口', async t => {
  const events = [
    failure,
    {id: 'created', event_type: 'visual_identity_candidate_created'},
    {id: 'accepted', kind: 'candidate_accepted', summary: '保留自然的深色短发 visual_identity_candidate_accepted'},
    {id: 'rejected', summary: 'visual_identity_candidate_rejected'},
    {id: 'ready', event_type: 'visual_identity_candidate_media_ready'},
    {id: 'future', summary: 'visual_identity_candidate_future_event'},
    {id: 'unknown-kind', kind: 'future_kind', summary: '已保留本次校对意见。'},
    {id: 'legacy', summary: '已接受初始规范参考', review: {source: 'manual', summary: '脸部轮廓一致。'}, asset_ids: ['private-asset']},
  ];
  const view = await mountTimeline(t, events);
  await view.click(button(view.host, '加载更多视觉记录'));
  await view.click(button(view.host, '加载更多视觉记录'));
  const entries = [...view.host.querySelectorAll('.r-timeline-entry')];
  assert.deepEqual(entries.map(entry => entry.querySelector('b').textContent), [
    '候选形象图片生成失败', '已创建候选形象', '已接受候选形象', '候选形象未通过评审',
    '候选形象图片已生成', '视觉身份有新的变化', '视觉身份有新的变化', '已接受初始规范参考',
  ]);
  assert.match(entries[0].querySelector('small').textContent, /2026-11-20 09:29.*北京时间/);
  assert.equal(entries[1].querySelector('p, small'), null);
  assert.match(entries[2].querySelector('p').textContent, /保留自然的深色短发/);
  assert.equal(entries[6].querySelector('p').textContent, '已保留本次校对意见。');
  assert.match(entries[7].textContent, /脸部轮廓一致。.*人工评审/s);
  assert.ok(button(entries[7], '查看当时的图片'));
  entries.forEach(assertCleanEvent);
});

test('失败事件展开后仅提供诊断下载，下载保留原始事件码及完整记录', async t => {
  const view = await mountTimeline(t, [failure]);
  const entry = view.host.querySelector('.r-timeline-entry');
  await view.click(button(entry, diagnosticTitle));
  assertCleanEvent(entry);
  const download = button(entry, '下载原始记录');
  assert.ok(download);

  const create = URL.createObjectURL;
  const revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype;
  const click = anchor.click;
  let blob, filename;
  URL.createObjectURL = value => { blob = value; return 'blob:visual-event-fixture'; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { filename = this.download; };
  try {
    await view.click(download);
    assert.equal(filename, 'fluctlight-record.json');
    const downloaded = JSON.parse(await blob.text());
    assert.equal(downloaded.summary, failureCode);
    assert.deepEqual(downloaded, failure);
    assertCleanEvent(entry);
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    anchor.click = click;
  }
});

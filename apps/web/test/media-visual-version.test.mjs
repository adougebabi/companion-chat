import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
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

const diagnosticTitle = '原始媒体记录（诊断）';
const technical = {
  id: 'res_media_private',
  revision: 1,
  source_ref: 'res_frozen_source',
  idempotency_key: 'media-command-private',
  metrics: {correlation_id: 'correlation-private'},
};
const instant = '2026-11-23T02:30:00.000Z';
const forbidden = /\bv\d+\b|第\s*\d+\s*版|(?:规范|格式|选择)版本|\b(?:id|revision|source_ref|idempotency_key|metrics|correlation_id|provider_job_id)\b|res_media_private|res_frozen_source|res_visual_style|res_event_private|res_avatar_history|req_media_private|media-command-private|correlation-private|provider-private|source-review-private|尚未记录|时间未记录|尚未填写/i;
const button = (root, label) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === label);

function assertBusinessView(root) {
  assert.doesNotMatch(root.textContent, forbidden);
  assert.equal(root.querySelector('pre'), null);
}

async function assertDownload(view, root, expected) {
  const trigger = button(root, diagnosticTitle);
  assert.ok(trigger, '显示诊断下载入口');
  await view.click(trigger);
  const download = button(trigger.closest('.product-disclosure'), '下载原始记录');
  assert.ok(download);
  const create = URL.createObjectURL;
  const revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype;
  const click = anchor.click;
  let blob, filename;
  URL.createObjectURL = value => { blob = value; return 'blob:media-test'; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { filename = this.download; };
  try {
    await view.click(download);
    assert.equal(filename, 'fluctlight-record.json');
    assert.deepEqual(JSON.parse(await blob.text()), expected);
    assertBusinessView(root);
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    anchor.click = click;
  }
}

async function mountMedia(t, tab, api, {query = {}, responses = {}} = {}) {
  const {createWorkspace} = await vite.ssrLoadModule('/src/state/workspace.mjs');
  const store = await vite.ssrLoadModule('/src/v3/proto/store.js');
  const {default: MediaPage} = await vite.ssrLoadModule('/src/react-app/pages/MediaPage.tsx');
  const {installMediaProvider} = await vite.ssrLoadModule('/src/v3/providers/media.ts');
  const {installInventoryProvider} = await vite.ssrLoadModule('/src/v3/providers/inventory.ts');
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
    if (Object.hasOwn(responses, path)) return Response.json(responses[path]);
    let response;
    const handled = await api.handler({
      url, path, method, body: {},
      json: (_res, row, status = 200) => { response = Response.json(row, {status}); },
    });
    assert.ok(handled, path);
    return response;
  };
  installMediaProvider(workspace.client, async () => {});
  installInventoryProvider(workspace.client);
  const view = await mountReact(vite, MediaPage, {
    workspace,
    props: {
      initialTab: tab,
      route: {path: '/c/A/media/' + tab, fid: 'A', query: new URLSearchParams(query)},
    },
  });
  t.after(async () => { await view.dispose(); workspace.dispose(); });
  return view;
}

test('视觉身份未建立时显示中文状态，省略空定义字段', async t => {
  const view = await mountMedia(t, 'identity', mediaFixture({scenario: 'empty'}));
  assert.ok([...view.host.querySelectorAll('h2')].some(node => node.textContent === '尚未建立视觉定义'));
  assert.doesNotMatch(view.host.textContent, /外貌要点|服装风格|一致性约束|参考图说明/);
  assertBusinessView(view.host);
});

test('已保存视觉定义保留业务说明和来源跳转，定义及评审原文仅下载', async t => {
  const api = mediaFixture();
  const state = api.state();
  Object.assign(state.brief.brief, {
    ...technical, appearance: '自然的深色短发', clothing: '',
    source_resource_id: 'res_visual_style', source_version: 'v1',
  });
  const timeline = {
    ...technical, id: 'res_event_private', stage: 'accepted', occurred_at: instant,
    summary: '已接受规范参考 res_event_private · v1',
    review: {source: 'source-review-private', summary: '脸部轮廓一致。'},
  };
  state.timeline = [timeline];
  const view = await mountMedia(t, 'identity', api);
  assert.match(view.host.textContent, /当前视觉定义.*外貌要点.*自然的深色短发/s);
  assert.doesNotMatch(view.host.textContent, /服装风格/);
  assert.match(view.host.textContent, /已接受规范参考.*脸部轮廓一致.*评审来源待确认/s);
  const source = [...view.host.querySelectorAll('a')].find(node => node.textContent === '查看定义来源');
  assert.equal(source?.getAttribute('href'), '#/studio/edit/res_visual_style');
  await assertDownload(view, source.closest('.panel'), state.brief);
  await assertDownload(view, view.host.querySelector('.r-timeline-entry'), timeline);
  await view.click(button(view.host, '生成新候选'));
  const dialog = document.querySelector('[role=dialog]');
  assert.match(dialog.textContent, /本次使用已保存的视觉定义/);
  assertBusinessView(dialog);
  assertBusinessView(view.host);
});

test('生成画面的物品和授权显示中文，结果摘要及失败原因可读，诊断下载保留原始记录', async t => {
  const request = {
    ...technical, id: 'req_media_private', fluctlight_id: 'A', status: 'failed',
    decision_reason: '生成服务暂时无法处理这幅画面。',
    payload: {
      prompt: '雨天的窗边', reason: '留下一幅雨景',
      _item_snapshot: {id: 'res_media_private', revision: 1, description: '蓝色针织开衫'},
      _place_snapshot: {id: 'place_window_private', revision: 1, name: '河边书店'},
      reference_asset: {purpose: 'scene', grant_id: 'grant-private'},
    },
  };
  const record = {
    fluctlight_id: 'A', request,
    media: {
      ...technical, status: 'failed', provider_job_id: 'provider-private',
      source: {requested_scene: {id: 'place_window_private', revision: 1, name: '河边旧书店'}},
    },
    asset: null, factual: false,
  };
  const view = await mountMedia(t, 'generate', mediaFixture(), {
    query: {request: request.id},
    responses: {
      '/api/fluctlights/A/media-generations': {items: [request], next_cursor: ''},
      ['/api/fluctlights/A/media-generations/' + request.id]: record,
      '/api/media-reference-grants': {items: [{...technical, purpose: 'scene', status: 'active'}], next_cursor: ''},
    },
  });
  await view.click(button(view.host, '关联登记物品 · 不关联物品'));
  await view.click(button(view.host, '参考图片 · 不使用参考图片'));
  assert.match(view.host.textContent, /登记的针织开衫/);
  assert.match(view.host.textContent, /场景参考 · 已授权/);
  const summary = view.host.querySelector('[aria-label="图片请求摘要"]');
  assert.match(summary.textContent, /画面描述雨天的窗边.*请求原因留下一幅雨景.*关联物品蓝色针织开衫.*画面地点河边旧书店.*参考用途场景参考/s);
  assert.match(view.host.textContent, /生成服务暂时无法处理这幅画面/);
  await assertDownload(view, view.host, record);
});

test('头像历史显示选择前后和保存原因，展开诊断也只提供原始记录下载', async t => {
  const api = mediaFixture();
  const row = {
    ...technical, id: 'res_avatar_history', reason: '换成窗边的照片', created_at: instant,
    before_value: {mode: 'canonical'},
    after_value: {mode: 'asset', asset_id: 'replica-landscape', asset_revision: 3},
  };
  api.state().avatarHistory = [row, {...technical, id: 'res_avatar_empty', reason: '', created_at: null}];
  const view = await mountMedia(t, 'avatar', api);
  const newest = view.host.querySelector('.r-timeline-entry');
  assert.match(newest.textContent, /已保存头像选择.*换成窗边的照片.*北京时间.*保存前：使用生成的头像.*保存后：使用选定的媒体图片/s);
  await view.click(button(newest, '保存后的图片'));
  assert.ok(newest.querySelector('img[alt="午后的窗边"]'));
  assert.equal(view.host.querySelectorAll('.r-timeline-entry')[1].querySelector('p, small'), null);
  await assertDownload(view, newest, row);
  assertBusinessView(view.host);
});

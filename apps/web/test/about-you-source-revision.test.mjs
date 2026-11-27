import assert from 'node:assert/strict';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { createServer } from './support/module-server.mjs';

const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  configFile: false,
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  esbuild: { jsx: 'automatic' },
  resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
  server: { middlewareMode: true, ws: false },
});
test.after(() => vite.close());

const { SourceFact } = await vite.ssrLoadModule('/src/react-app/shared/PeopleSourceSheet.jsx');
const row = {
  attribute: 'occupation',
  value: '插画师 <绘本 & 漫画>',
  epistemic_kind: 'owner_defined',
  transition_kind: 'correct',
  reason: '补充工作方向',
  revision: 840731,
  id: 'private-fact-id',
  source_ref: 'private-source-ref',
  idempotency_key: 'private-request-key',
  metrics: { confidence: 'private-metric-value' },
};

function renderFact(record) {
  return JSDOM.fragment(renderToStaticMarkup(React.createElement(SourceFact, { row: record })));
}

test('关于你了解记录隐藏修订版本和缺失时间，诊断下载保留完整原记录', () => {
  const view = renderFact(row);
  assert.equal(view.querySelector('h3').textContent, '职业 / 身份');
  assert.deepEqual(
    [...view.querySelectorAll('.profile-key-value')].map((fact) => fact.textContent),
    ['记录内容插画师 <绘本 & 漫画>', '资料来源主动填写', '修改含义纠正了自填资料', '修改说明补充工作方向'],
  );
  assert.doesNotMatch(view.textContent, /修订版本|840731|记录时间|尚未记录|\bid\b|revision|source_ref|idempotency|metrics|private-/i);

  const links = view.querySelectorAll('a[download]');
  assert.equal(links.length, 1);
  const [link] = links;
  assert.equal(link.textContent, '原始了解记录（诊断）');
  assert.equal(link.download, 'people-source.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = link.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  const downloaded = JSON.parse(decodeURIComponent(href.slice(prefix.length)));
  assert.equal(downloaded.revision, row.revision);
  assert.deepEqual(downloaded, row);
});

test('关于你了解记录优先展示生效时间，缺少生效时间时展示创建时间', () => {
  const created_at = '2026-11-23T01:15:00Z';
  const effective_at = '2026-11-23T03:45:00Z';
  const effective = renderFact({ ...row, created_at, effective_at });
  assert.match(effective.textContent, /记录时间2026-11-23 11:45 \(北京时间\)/);
  assert.doesNotMatch(effective.textContent, /09:15/);
  const created = renderFact({ ...row, created_at });
  assert.match(created.textContent, /记录时间2026-11-23 09:15 \(北京时间\)/);
});

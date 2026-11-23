import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import ts from 'typescript';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const require = createRequire(import.meta.url);
const h = React.createElement;
const Box = ({ children }) => h('div', null, children);
const Button = ({ children, asChild }) => asChild ? children : h('button', null, children);
const Disclosure = ({ title, children }) => h('details', { open: true }, h('summary', null, title), children);
const Frame = ({ title, children, footer }) => h('section', null, h('h2', null, title), children, footer);
const internal = {
  actor_id: 'private-actor-913', context_version: 'private-context-914',
  revision: 'private-revision-915', factsRevision: 'private-facts-916',
  source_ref: 'private-source-917', metrics: { trust: 'private-metric-918' },
  idempotencyKey: 'private-key-919', id: 'private-id-920',
};
const background = {
  full_name: '林小满', name: '小满', age: '28', gender: '女', occupation: '插画师',
  relationship_description: '老朋友', wants_you_to_know: '周末画画', interests: '散步',
  preferences: '慢慢聊', habits: '早睡', rules: '不打扰工作', meeting_confirmed: false,
  personality: '安静', shared_information: '喜欢海边', timezone: 'Asia/Shanghai',
  location: null, location_scope: '', relationship_distance: '   ', ...internal,
};
const pending = { input: { background, operation: 'correct', reason: '更新职业', ...internal }, ...internal };
const latest = { background: { ...background, name: '最新称呼' }, facts: [{ ...internal }], history: [], ...internal };

// Compile only the components under test. Hooks use the real React SSR implementation;
// workspace/network and command state are supplied locally, with no app bootstrap.
function harness(state = {}) {
  const cache = new Map(), downloads = [];
  let capturedAction;
  const source = { items: [latest], ready: true, busy: false };
  const mocks = {
    react: React,
    'lucide-react': Object.fromEntries(['ArrowRightIcon', 'ChevronRightIcon', 'FileClockIcon', 'HeartIcon', 'RefreshCwIcon', 'PencilIcon'].map((key) => [key, () => null])),
    '@/components/ui/button': { Button },
    '@/components/ui/progress': { Progress: Box },
    '@/components/ui/alert-dialog': { AlertDialogAction: Button, AlertDialogCancel: Button },
    sonner: { toast: { error() {} } },
    '../controls': { Surface: Box, Badge: Box, Disclosure, FieldInput: Box, Textarea: Box, ChoiceSelect: Box },
    '../components': { DetailDrawer: Frame },
    '../page': { useWorkspace: () => ({ state: { session: { actorId: 'owner' } } }) },
    '@/state/domain-command.mjs': { createDomainCommand: () => ({
      subscribe: () => () => {}, getSnapshot: () => state,
    }) },
    './WorkspaceDialog': { default: Frame },
    './Presentation': { InlineFeedback: Box, ReaderFeedback: () => null, EmptyState: Box },
    './ConfirmationFrame': { default: Frame },
    './CommandSyncAction': { default: () => null },
    './LoadingState': { default: () => null },
    './WorkspaceInfo': { default: Box },
    './CharacterAvatar': { default: () => null },
    './useWorkspaceRead': { default: () => source },
    '@/v3/providers/real.ts': { readBackground: () => { throw Error('unexpected backend read'); } },
    '@/v3/presentation-time.mjs': { userInstant: (value) => value },
  };
  const files = {
    './AboutYouPanel': 'react-app/shared/AboutYouPanel.jsx',
    './WorkspaceCommandDialog': 'react-app/shared/WorkspaceCommandDialog.jsx',
    './PeopleSourceSheet': 'react-app/shared/PeopleSourceSheet.jsx',
    './PeopleParts': 'react-app/shared/PeopleParts.jsx',
    './JsonDetails': 'react-app/shared/JsonDetails.tsx',
    './RecordView': 'react-app/shared/RecordView.tsx',
    '@/approved-ui/people-presentation.mjs': 'approved-ui/people-presentation.mjs',
    '../lib/actor-user-background.ts': 'lib/actor-user-background.ts',
  };
  function load(id) {
    if (mocks[id]) return { __esModule: true, ...mocks[id] };
    if (cache.has(id)) return cache.get(id);
    assert.ok(files[id], `Unexpected import: ${id}`);
    const file = new URL(`../src/${files[id]}`, import.meta.url);
    const code = ts.transpileModule(readFileSync(file, 'utf8'), {
      fileName: file.pathname.replace(/\.mjs$/, '.js'),
      compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true, allowJs: true },
    }).outputText;
    const module = { exports: {} };
    new Function('require', 'module', 'exports', code)((name) => name === 'react/jsx-runtime' ? require(name) : load(name), module, module.exports);
    let result = module.exports;
    if (id === './JsonDetails') {
      const Component = result.default;
      result = { __esModule: true, ...result, default: (props) => { downloads.push(props); return h(Component, props); } };
    }
    cache.set(id, result);
    return result;
  }
  const about = load('./AboutYouPanel');
  // Capture the actual page action without opening an editor or triggering effects.
  mocks.react = { ...React, useState: (initial) => [initial === false ? true : typeof initial === 'function' ? false : initial, () => {}] };
  cache.delete('./AboutYouPanel');
  mocks['./WorkspaceCommandDialog'] = { default: ({ action }) => { capturedAction = action; return null; } };
  const Editable = load('./AboutYouPanel').default;
  renderToStaticMarkup(h(Editable, { model: {
    w: {}, fid: 'character', owner: 'owner', actorId: 'owner', c: { name: '她' },
    profile: { items: [] }, live: () => true,
  } }));
  delete mocks['./WorkspaceCommandDialog'];
  mocks.react = React;
  return { about, load, downloads, action: capturedAction };
}
const forbidden = /actor[ _]id|context[ _]version|revision|版本|factsRevision|source[ _]ref|idempotency|metrics|private-[\w-]+|尚未记录/i;
const render = (Component, props) => renderToStaticMarkup(h(Component, props));

test('关于你恢复与冲突弹窗：中文摘要、仅下载诊断和完整原记录', () => {
  const fixture = harness({ pending: { value: pending }, review: { object: latest } });
  const html = render(fixture.load('./WorkspaceCommandDialog').default, { action: { ...fixture.action, fields: () => null } });
  assert.doesNotMatch(html, forbidden);
  for (const text of ['姓名', '林小满', '称呼', '小满', '最新称呼', '年龄', '28', '性别', '女', '职业 / 身份', '插画师', '你填写的关系', '老朋友', '你想让她知道的事', '周末画画', '喜好', '散步', '交流偏好', '慢慢聊', '习惯', '早睡', '边界', '不打扰工作', '之前写错了', '更新职业', '核对最新资料', '原始提交内容（诊断）', '最新记录与来源（诊断）']) assert.ok(html.includes(text), text);
  assert.equal((html.match(/下载原始记录/g) || []).length, 2);
  assert.deepEqual(fixture.downloads.map(({ value, downloadOnly, title }) => ({ value, downloadOnly, title })), [
    { value: pending, downloadOnly: true, title: '原始提交内容（诊断）' },
    { value: latest, downloadOnly: true, title: '最新记录与来源（诊断）' },
  ]);
});

test('关于你摘要：空值无占位，保留 false、0 和用户正文，不遍历嵌套内部对象', () => {
  const { about } = harness();
  assert.equal(render(about.AboutYouSummary, { background: { name: '', age: null, preferences: '  ', rules: undefined } }), '<div class="companion-fact-grid"></div>');
  const html = render(about.AboutYouSummary, { background: { age: 0, meeting_confirmed: false, name: '<小满 & 朋友>', preferences: { ...internal } } });
  assert.match(html, /年龄.*0/);
  assert.match(html, /是否已约定线下见面.*否/);
  assert.match(html, /&lt;小满 &amp; 朋友&gt;/);
  assert.doesNotMatch(html, forbidden);
  assert.doesNotMatch(html, /交流偏好/);
});

test('未提供页面摘要的命令仅下载原提交和最新对象，并保留基线核对入口', () => {
  const fixture = harness({ pending: { value: pending }, review: { object: latest } });
  const html = render(fixture.load('./WorkspaceCommandDialog').default, {
    action: { identity: 'other', title: '更新操作资料', fields: () => null },
  });
  for (const text of ['更新操作资料', '这项操作的完整内容可在诊断记录中下载。', '核对最新资料', '已核对，采用最新基线']) assert.ok(html.includes(text), text);
  assert.doesNotMatch(html, forbidden);
  assert.equal((html.match(/下载原始记录/g) || []).length, 2);
  assert.deepEqual(fixture.downloads.map(({ value, title, downloadOnly }) => ({ value, title, downloadOnly })), [
    { value: pending, title: '原始提交记录（诊断）', downloadOnly: true },
    { value: latest, title: '原始核对记录（诊断）', downloadOnly: true },
  ]);
});

test('关于你主视图与 PeopleSourceSheet 保持业务字段阅读和完整来源仅下载', () => {
  const fixture = harness();
  const html = render(fixture.about.default, { model: {
    w: {}, fid: 'character', owner: 'owner', actorId: 'owner', c: { name: '她' },
    profile: { items: [] }, live: () => true,
  } });
  assert.match(html, /姓名/);
  assert.match(html, /林小满/);
  assert.match(html, /最新称呼/);
  assert.doesNotMatch(html, forbidden);
  const sheet = render(fixture.load('./PeopleSourceSheet').default, { open: true, source: latest, title: '资料来源', children: '中文业务摘要' });
  const download = /href="data:application\/json;charset=utf-8,([^"]+)"/.exec(sheet);
  assert.ok(download);
  assert.deepEqual(JSON.parse(decodeURIComponent(download[1])), latest);
  assert.doesNotMatch(sheet.replace(/href="[^"]+"/g, ''), forbidden);
  assert.match(sheet, /download="people-source.json"/);
  assert.match(sheet, /中文业务摘要/);
  assert.doesNotMatch(sheet, /<pre|<dl/);
});

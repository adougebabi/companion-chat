import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {React, installDOM, mountReact} from './helpers/react-dom.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom', logLevel: 'silent', server: {middlewareMode: true},
});
const {SidebarProvider, Sidebar, SidebarTrigger} = await vite.ssrLoadModule('/src/components/ui/sidebar.tsx');
const {default: WorkspaceNavigation} = await vite.ssrLoadModule('/src/react-app/shared/WorkspaceNavigation.tsx');
test.after(async () => {await vite.close(); env.restore();});

test('侧栏展开及收起时，用户区独立于导航滚动区，页头和底部不收缩', async () => {
  const Component = () => React.createElement(SidebarProvider, {mobileBreakpoint: 701},
    React.createElement(Sidebar, {collapsible: 'icon', variant: 'inset'},
      React.createElement(WorkspaceNavigation, {page: '管理中心', character: {id: 'test-character', name: '林青'}, onNavigate() {},
        accountMenu: React.createElement('button', {'aria-label': '账户菜单'}, '账户菜单')})),
    React.createElement(SidebarTrigger));
  const view = await mountReact(vite, Component);
  try {
    const checkLayout = () => {
      const header = view.host.querySelector('[data-slot="sidebar-header"]');
      const content = view.host.querySelector('[data-slot="sidebar-content"]');
      const footer = view.host.querySelector('[data-slot="sidebar-footer"]');
      assert.equal(header.parentElement, content.parentElement);
      assert.equal(content.nextElementSibling, footer);
      assert.equal(content.contains(footer), false);
      assert.ok(header.classList.contains('shrink-0'));
      assert.ok(footer.classList.contains('shrink-0'));
      assert.ok(footer.classList.contains('border-t'));
      assert.ok(footer.classList.contains('bg-sidebar'));
      for (const name of ['min-h-0', 'flex-1', 'overflow-x-hidden', 'overflow-y-auto']) {
        assert.ok(content.classList.contains(name), name);
      }
      assert.equal(content.querySelectorAll('[data-slot="sidebar-menu-button"]').length, 16);
      assert.equal(content.querySelector('[aria-current="page"]').textContent, '管理中心');
      return footer;
    };
    assert.match(checkLayout().textContent, /我的空间/);
    await view.click(view.host.querySelector('[data-slot="sidebar-trigger"]'));
    assert.equal(view.host.querySelector('[data-state]').dataset.state, 'collapsed');
    assert.equal(checkLayout().querySelector('button').textContent, '账户菜单');
    assert.equal(checkLayout().querySelector('.replica-owner-avatar'),null,'compact navigation preserves the account action instead of an inert avatar');
  } finally {await view.dispose();}
});

test('底部使用与桌面侧栏一致的不透明底色和实线顶部分隔', async () => {
  const style = document.createElement('style');
  style.textContent = await readFile(new URL('../src/styles/workspace-replica.css', import.meta.url), 'utf8');
  document.head.append(style);
  try {
    const rules = [...style.sheet.cssRules];
    const footer = rules.find(rule => rule.selectorText === '.replica-nav-footer').style;
    const sidebar = rules.find(rule => rule.selectorText?.endsWith('> [data-slot="sidebar-inner"]')).style;
    assert.equal(footer.background, 'rgb(251, 252, 248)');
    assert.equal(footer.background, sidebar.background);
    assert.equal(footer.borderTopWidth, '1px');
    assert.equal(footer.borderTopStyle, 'solid');
    assert.equal(footer.borderTopColor, 'rgb(226, 231, 222)');
  } finally {style.remove();}
});

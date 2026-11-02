import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile, readdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {React, act, installDOM, mountReact} from './helpers/react-dom.mjs';

const env = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../', import.meta.url)), appType:'custom', logLevel:'silent', server:{middlewareMode:true}});
const {default:DomainPrimitives} = await vite.ssrLoadModule('/src/react-app/shared/DomainPrimitives.tsx');
const {Button, SearchForm} = await vite.ssrLoadModule('/src/v3/proto/components/forms.js');
test.after(async () => {await vite.close(); env.restore();});

function Harness({html, prepare = () => {}}) {
  const [root, setRoot] = React.useState(null);
  const ref = React.useCallback(node => {
    if (node) {node.innerHTML = html; prepare(node); setRoot(node);}
  }, []);
  return React.createElement(React.Fragment, null, React.createElement('section', {ref}), React.createElement(DomainPrimitives, {root}));
}
const controls = host => [...host.querySelectorAll('[data-slot="button"]')].filter(node => !node.closest('[data-domain-carrier]'));

test('domain declarations render real shadcn Button variants and sizes without custom button classes', async () => {
  const cases = [
    ['outline', 'default'], ['default', 'default'], ['destructive', 'sm'],
    ['ghost', 'sm'], ['ghost', 'icon'], ['secondary', 'default'],
    ['secondary', 'lg'], ['link', 'xs'],
  ];
  const view = await mountReact(vite, Harness, {props:{html:cases.map(([variant, size]) => Button({label:variant + '/' + size, variant, size})).join('')}});
  try {
    const buttons = controls(view.host);
    assert.equal(buttons.length, cases.length);
    for (const [index, [variant, size]] of cases.entries()) {
      assert.equal(buttons[index].dataset.variant, variant);
      assert.equal(buttons[index].dataset.size, size);
      assert.equal(buttons[index].classList.contains('group/button'), true);
      assert.equal(buttons[index].classList.contains('btn'), false);
      assert.equal(buttons[index].textContent, variant + '/' + size);
    }
    assert.match(buttons[2].className, /bg-destructive/);
    assert.match(buttons[3].className, /h-8/);
    assert.match(buttons[4].className, /size-9/);
    assert.match(buttons[6].className, /h-10/);
  } finally {await view.dispose();}
});

test('ported actions preserve original submitter, one click, current text, hidden and disabled state', async () => {
  let source, form, clicks = 0, submissions = 0, submitter;
  const view = await mountReact(vite, Harness, {props:{
    html:'<form><input name="draft" value="保留正文">' + Button({label:'保存', variant:'default', size:'sm', type:'submit', attrs:{name:'action', value:'save'}}) + '</form>',
    prepare:root => {
      form = root.querySelector('form'); source = form.querySelector('button');
      source.onclick = () => clicks++;
      form.onsubmit = event => {event.preventDefault(); submissions++; submitter = event.submitter;};
    },
  }});
  try {
    const action = controls(view.host)[0];
    await view.click(action);
    assert.equal(clicks, 1); assert.equal(submissions, 1); assert.equal(submitter, source);
    assert.deepEqual([...new window.FormData(form, submitter)], [['draft','保留正文'], ['action','save']]);
    await act(async () => {source.disabled = true; source.textContent = '正在保存';});
    await view.flush(); assert.equal(action.disabled, true); assert.equal(action.textContent, '正在保存');
    await view.click(action); assert.equal(clicks, 1); assert.equal(submissions, 1);
    await act(async () => {source.hidden = true;}); await view.flush();
    assert.equal(action.closest('[data-domain-react-control]').hidden, true);
    await act(async () => {source.hidden = false; source.disabled = false; source.dataset.domainVariant = 'destructive'; source.dataset.domainSize = 'icon';});
    await view.flush(); assert.equal(action.dataset.variant, 'destructive'); assert.equal(action.dataset.size, 'icon');
    source.focus(); assert.equal(document.activeElement, action);
  } finally {await view.dispose();}
});

test('Button links preserve href, target, rel, download and original listeners while redraw releases old controls', async () => {
  let root, source, clicks = 0;
  const view = await mountReact(vite, Harness, {props:{
    html:'<a data-domain-ui="button" data-domain-variant="ghost" data-domain-size="sm" href="/export" target="_blank" rel="noopener noreferrer" download="资料.json" aria-label="导出资料">下载资料</a>',
    prepare:node => {root = node; source = node.querySelector('a'); source.onclick = event => {event.preventDefault(); clicks++;};},
  }});
  try {
    const link = controls(view.host)[0];
    assert.equal(link.tagName, 'A');
    for (const name of ['href','target','rel','download','aria-label']) assert.equal(link.getAttribute(name), source.getAttribute(name));
    await view.click(link); assert.equal(clicks, 1);
    await act(async () => {root.innerHTML = SearchForm({submitLabel:'重新搜索', buttonVariant:'default', buttonSize:'sm'});});
    await view.flush();
    assert.equal(controls(view.host).length, 1);
    assert.equal(controls(view.host)[0].textContent, '重新搜索');
    assert.equal(controls(view.host)[0].dataset.variant, 'default');
    assert.equal(controls(view.host)[0].dataset.size, 'sm');
    assert.equal(link.isConnected, false);
  } finally {await view.dispose();}
});

async function sourceFiles(folder) {
  const files = [];
  for (const entry of await readdir(folder, {withFileTypes:true})) {
    const url = new URL(entry.name + (entry.isDirectory() ? '/' : ''), folder);
    if (entry.isDirectory()) files.push(...await sourceFiles(url));
    else if (/\.[cm]?[jt]sx?$/.test(entry.name)) files.push(url);
  }
  return files;
}
test('production source has no custom btn classes and every domain button declares variant and size', async () => {
  let declarations = 0;
  for (const file of await sourceFiles(new URL('../src/', import.meta.url))) {
    const source = await readFile(file, 'utf8');
    assert.doesNotMatch(source, /\bclass(?:Name)?\s*=\s*["'][^"'\n]*\bbtn\b/, file.pathname);
    for (const [tag] of source.matchAll(/<(?:a|button)\b[^>]*data-domain-ui="button"[^>]*/g)) {
      declarations++;
      assert.match(tag, /data-domain-variant=/, file.pathname);
      assert.match(tag, /data-domain-size=/, file.pathname);
    }
  }
  assert.ok(declarations > 600, 'scan must include the actual JS domain controllers and their shared factories');
  assert.doesNotMatch(await readFile(new URL('../src/styles/domain-tools.css', import.meta.url), 'utf8'), /\.btn\b/);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {RawRecordDownload} from '../src/v3/proto/components/records.js';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

const title = '原始记录（诊断）';
const value = {source_ref: 'private-source', summary: '诊断记录 & 原文'};
const defaultFilename = 'fluctlight-record.json';
const customFilename = `原始记录 "&<复核>'".json`;

test('JsonDetails uses the default filename and preserves a custom filename', async t => {
  const vite = await createServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    envFile: false, appType: 'custom', logLevel: 'silent', server: {middlewareMode: true},
  });
  t.after(() => vite.close());
  const {default: JsonDetails} = await vite.ssrLoadModule('/src/react-app/shared/JsonDetails.tsx');
  const props = {title, value, downloadOnly: true};
  const view = await mountReact(vite, JsonDetails, {props});
  const button = text => [...view.host.querySelectorAll('button')].find(node => node.textContent === text);
  const downloads = [];
  t.mock.method(window.HTMLAnchorElement.prototype, 'click', function () {
    downloads.push(this.download);
  });

  try {
    await view.click(button(title));
    await view.click(button('下载原始记录'));
    await view.update({...props, filename: customFilename});
    await view.click(button('下载原始记录'));
    assert.deepEqual(downloads, [defaultFilename, customFilename]);
  } finally {
    await view.dispose();
  }
});

test('RawRecordDownload uses the default filename and escapes a custom filename', () => {
  const env = installDOM();
  try {
    const host = document.createElement('div');
    host.innerHTML = RawRecordDownload(value, title);
    assert.equal(host.querySelector('a').download, defaultFilename);

    const html = RawRecordDownload(value, title, customFilename);
    assert.match(html, /download="原始记录 &quot;&amp;&lt;复核&gt;&#39;&quot;\.json"/);
    host.innerHTML = html;
    const link = host.querySelector('a');
    assert.equal(link.download, customFilename);
    assert.equal(link.textContent, title);
    const prefix = 'data:application/json;charset=utf-8,';
    assert.ok(link.href.startsWith(prefix));
    assert.deepEqual(JSON.parse(decodeURIComponent(link.href.slice(prefix.length))), value);
  } finally {
    env.restore();
  }
});

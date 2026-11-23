import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';
import {mediaFixture} from './fixtures/media-fixture.mjs';

const env = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../', import.meta.url)), envFile:false, appType:'custom', logLevel:'silent', server:{middlewareMode:true}});
const {default:Record} = await vite.ssrLoadModule('/src/react-app/shared/VisualIdentityRecord.jsx');
const button = (host, text) => [...host.querySelectorAll('button')].find(node => node.textContent.trim() === text);
const forbidden = /adapter_version|chest-cup|fluctlight_id|renderer_constraints|schema_version|private-|unknown_extension|sql:|null/;
const diagnosticTitle = '原始媒体记录（诊断）';
const raw = {
  schema_version:'visual-identity.v1', current_revision:0, id:'private-identity', fluctlight_id:'private-character',
  adapter_version:'chest-cup-adapter.v1', identity_snapshot:{},
  renderer_constraints:{adapter_version:'chest-cup-adapter.v1',chest_cup:null}, generated_reviews:{}, session:null,
};
test.after(async () => {await vite.close(); env.restore();});
async function mount(value) {
  const view = await mountReact(vite, Record, {props:{value}});
  assert.equal(view.host.querySelector('dl'), null, 'summary starts collapsed');
  return view;
}
function facts(view) {
  return Object.fromEntries([...view.host.querySelectorAll('dl > div')].map(node => [node.querySelector('dt').textContent, node.querySelector('dd').textContent]));
}
test('a saved visual record with no business fields shows only the diagnostic entry', async () => {
  const view = await mount(raw);
  try {
    assert.ok(button(view.host, diagnosticTitle));
    assert.deepEqual(facts(view), {});
    assert.doesNotMatch(view.host.textContent, /规范版本|记录格式版本|第 \d+ 版|渲染约束|会话状态|评审状态|尚未记录/);
    assert.doesNotMatch(view.host.textContent, forbidden);
    assert.equal(view.host.querySelector('pre'), null);
  } finally {await view.dispose();}
});
test('known constraints and current attempt preserve zero and false without exposing unknown values', async () => {
  const view = await mount({...raw,current_revision:3,renderer_constraints:{chest_cup:'not_applicable',chest_lora_weight:0,chest_lora_applicable:false,unknown_extension:'private-constraint'},session:{status:'completed',current_attempt:2,last_error:'sql: private-error',attempts:[{attempt_number:1,decision:'rejected'},{attempt_number:2,decision:'accepted'}]},generated_reviews:{avatar:{status:'rejected'},character_sheet:{status:'awaiting_review'}}});
  try {
    await view.click(button(view.host, '渲染约束与视觉会话'));
    assert.deepEqual(facts(view), {'渲染约束':'已记录身体比例约束','胸部尺寸':'不适用','比例调整权重':'0','比例调整是否适用':'否','会话状态':'本次准备已完成','候选评审状态':'已通过','头像评审状态':'未通过','角色设定图评审状态':'待评审'});
    assert.doesNotMatch(view.host.textContent, forbidden);
    await view.update({value:{...raw,schema_version:'private-schema',session:{status:'private-status',current_attempt:1,attempts:[{attempt_number:1,status:'private-review'}]},renderer_constraints:{chest_cup:'private-size'}}});
    assert.doesNotMatch(view.host.textContent, forbidden);
    assert.equal(facts(view)['记录格式版本'], undefined);
    assert.equal(facts(view)['候选评审状态'], '评审状态待确认');
  } finally {await view.dispose();}
});
test('original record downloads unchanged without rendering raw JSON', async () => {
  const view = await mount(raw);
  const create = URL.createObjectURL, revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype, click = anchor.click;
  let blob, filename;
  URL.createObjectURL = value => {blob = value; return 'blob:visual-record';};
  URL.revokeObjectURL = () => {};
  anchor.click = function () {filename = this.download;};
  try {
    await view.click(button(view.host, diagnosticTitle));
    await view.click(button(view.host,'下载原始记录'));
    assert.deepEqual(JSON.parse(await blob.text()), raw);
    assert.equal(filename, '视觉身份原始记录.json');
    assert.doesNotMatch(view.host.textContent, forbidden);
    assert.equal(view.host.querySelector('pre'), null);
  } finally {URL.createObjectURL=create; URL.revokeObjectURL=revoke; anchor.click=click; await view.dispose();}
});
test('unavailable identity omits empty fields and diagnostic controls', async () => {
  const view = await mount(undefined);
  try {
    assert.equal(view.host.textContent, '');
    assert.equal(button(view.host, diagnosticTitle), undefined);
    assert.equal(button(view.host,'下载原始记录'), undefined);
  } finally {await view.dispose();}
});
for (const scoped of [false, true]) test(`${scoped ? 'character route' : 'media tab'} uses safe summary and performs only reads`, async () => {
  const {createWorkspace} = await vite.ssrLoadModule('/src/state/workspace.mjs');
  const store = await vite.ssrLoadModule('/src/v3/proto/store.js');
  const {default:Media} = await vite.ssrLoadModule('/src/react-app/pages/MediaPage.tsx');
  const api = mediaFixture({scenario:'empty'});
  Object.assign(api.state().visual, raw, {fluctlight_id:'A'});
  const w = createWorkspace();
  store.replaceSnapshot({...store.get(),session:{actorId:'media-owner'},characters:{A:{id:'A',name:'林青',status:'active'}},ui:{charId:'A'}});
  w.client.fetcher = async (input, init={}) => {
    assert.equal(init.method || 'GET','GET','viewing never writes or generates');
    const url = new URL(input,'http://media.test');
    let response;
    const handled = await api.handler({url,path:url.pathname,method:'GET',body:{},json:(_res,row,status=200)=>{response=Response.json(row,{status});}});
    assert.ok(handled, url.pathname);
    return response;
  };
  const view = await mountReact(vite,Media,{workspace:w,props:{route:{path:scoped?'/c/A/media/identity':'/media',fid:scoped?'A':undefined,query:new URLSearchParams()},initialTab:scoped?'identity':undefined}});
  try {
    if (!scoped) {
      await act(async () => [...view.host.querySelectorAll('[role=tab]')].find(node=>node.textContent.startsWith('视觉身份')).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));
      await view.flush();
    }
    await view.click(button(view.host, diagnosticTitle));
    assert.match(view.host.textContent,/尚未建立视觉定义/);
    assert.doesNotMatch(view.host.textContent,/规范版本|第 \d+ 版|尚未记录/);
    assert.ok(button(view.host, '下载原始记录'));
    assert.doesNotMatch(view.host.textContent,forbidden);
    assert.equal(view.host.querySelector('pre'),null);
    assert.equal(api.calls.filter(row=>row.method!=='GET').length,0);
  } finally {await view.dispose(); w.dispose();}
});

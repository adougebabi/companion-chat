import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

const env = installDOM();
const vite = await createServer({root:fileURLToPath(new URL('../', import.meta.url)), envFile:false, appType:'custom', logLevel:'silent', server:{middlewareMode:true}});
test.after(async () => {await vite.close(); env.restore();});
const {default:DiaryPage, DiaryProvenance} = await vite.ssrLoadModule('/src/react-app/pages/DiaryPage.jsx');
const button = (host, text) => [...host.querySelectorAll('button')].find(node => node.textContent.trim() === text);
const record = () => ({
  id:'private-diary', fluctlight_id:'A', date:'2026-10-26', timezone:'Asia/Shanghai', status:'ready', source:'agent',
  body:'今天读完了一本书。', revision:31, active_attempt_id:'private-attempt',
  annotation:{note:'这段话我很喜欢。', favorite:true, revision:47},
  refs:['message:private-message'],
  source_snapshot:{sources:[{ref:'message:private-message', kind:'remote_communication'}, {ref:'event:private-event', kind:'event'}]},
  attempts:[{id:'private-attempt', attempt:3, status:'ready', created_at:'2026-10-26T14:00:00Z', task_id:'private/task'}],
  has_more_attempts:true,
});
const clean = host => {
  assert.doesNotMatch(host.textContent, /active[_ ]attempt[_ ]id|annotation|favorite|revision|尚未记录|private-|版本|\bbody\b|\bnote\b/);
  assert.equal(host.querySelector('.record-facts, pre, code'), null);
};

test('diary detail keeps prose and annotations above Chinese provenance and downloads the unchanged complete record', async () => {
  const entry = record(), state = proxy({session:{actorId:'owner'}, selectedId:'A', characters:[{id:'A', name:'林青', status:'正常'}]});
  const workspace = {
    state, current:{get value(){return state.characters[0];}},
    subscribe:fn => subscribe(state,fn), getSnapshot:() => snapshot(state),
    isCurrent:(owner,fid) => owner === 'owner' && fid === 'A', currentScope:() => JSON.stringify(['owner','A']),
    client:{listDiary:async () => ({items:[entry], next_cursor:''}), diary:async () => entry, diaryReplies:async () => ({items:[], next_cursor:''})},
  };
  const v = await mountReact(vite, DiaryPage, {workspace});
  const blobs = [], links = [], create = URL.createObjectURL, revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype, click = anchor.click;
  URL.createObjectURL = blob => {blobs.push(blob); return 'blob:diary-fixture';};
  URL.revokeObjectURL = () => {};
  anchor.click = function(){links.push(this.download);};
  try {
    const article = v.host.querySelector('[aria-label="日记正文"]');
    assert.ok(article);
    assert.equal(article.querySelector('.lp-paper-body').textContent, entry.body);
    assert.match(article.textContent, /已收藏/);
    assert.equal(article.querySelector('.lp-annotation p').textContent, entry.annotation.note);
    await v.click(button(article, '生成尝试、素材与来源'));
    await v.click(button(article, '原始日记记录（诊断）'));
    clean(article);
    assert.equal(article.textContent.split(entry.body).length - 1, 1);
    assert.equal(article.textContent.split(entry.annotation.note).length - 1, 1);
    assert.match(article.textContent, /第 3 次整理 · 已经写下/);
    assert.equal(article.querySelector('time').dateTime, entry.attempts[0].created_at);
    assert.match(article.querySelector('time').textContent, /2026.*10.*26.*22:00/);
    assert.match(article.textContent, /可用素材 2 条（远程交流 1 条）/);
    assert.match(article.textContent, /正文引用来源 1 条/);
    assert.match(article.textContent, /更早记录保留在实际任务诊断/);
    assert.equal([...article.querySelectorAll('a')].find(node => node.textContent === '查看原任务').getAttribute('href'), '#/diag/task/private%2Ftask');
    await v.click(button(article, '下载原始记录'));
    assert.deepEqual(links, ['fluctlight-record.json']);
    assert.deepEqual(JSON.parse(await blobs[0].text()), entry);
    clean(article);
  } finally {
    URL.createObjectURL = create; URL.revokeObjectURL = revoke; anchor.click = click;
    await v.dispose();
  }
});

test('diary provenance omits missing materials, references, attempt time and empty annotation placeholders', async () => {
  const entry = {...record(), refs:[], source_snapshot:null, annotation:{note:'', favorite:false, revision:0}, attempts:[{attempt:1,status:'failed'}], has_more_attempts:false};
  const v = await mountReact(vite, DiaryProvenance, {props:{entry}});
  try {
    await v.click(button(v.host, '生成尝试、素材与来源'));
    await v.click(button(v.host, '原始日记记录（诊断）'));
    clean(v.host);
    assert.match(v.host.textContent, /第 1 次整理 · 整理未完成/);
    assert.doesNotMatch(v.host.textContent, /可用素材|正文引用来源|远程交流|已收藏|你的批注|更早记录/);
    assert.equal(v.host.querySelector('time, a'), null);
    assert.ok(button(v.host, '下载原始记录'));
  } finally {await v.dispose();}
});

test('diary attempt summaries translate all known states and keep unknown states internal', async () => {
  const states = {pending:'等待整理',running:'正在回顾',ready:'已经写下',no_content:'无正文',failed:'整理未完成',inactive:'尚未运行',private_status:'状态待确认'};
  const entry = {...record(), attempts:Object.keys(states).map((status,index) => ({id:String(index),attempt:index+1,status}))};
  const v = await mountReact(vite, DiaryProvenance, {props:{entry}});
  try {
    await v.click(button(v.host, '生成尝试、素材与来源'));
    for (const [index,label] of Object.values(states).entries()) assert.ok(v.host.textContent.includes(`第 ${index+1} 次整理 · ${label}`));
    assert.doesNotMatch(v.host.textContent, /private_status|undefined|尚未记录/);
  } finally {await v.dispose();}
});

test('diary provenance counts references without requiring a source snapshot or guessing source types', async () => {
  const entry = {...record(), refs:['event:one','event:one','message:two'], source_snapshot:{sources:[{kind:'private_unknown_kind',ref:'other:three'}]}, attempts:[]};
  const v = await mountReact(vite, DiaryProvenance, {props:{entry}});
  try {
    await v.click(button(v.host, '生成尝试、素材与来源'));
    assert.match(v.host.textContent, /可用素材 1 条/);
    assert.match(v.host.textContent, /正文引用来源 2 条/);
    assert.doesNotMatch(v.host.textContent, /private_unknown_kind|远程交流/);
    await v.update({entry:{...entry,source_snapshot:undefined}});
    assert.match(v.host.textContent, /正文引用来源 2 条/);
    assert.doesNotMatch(v.host.textContent, /可用素材|尚未记录/);
  } finally {await v.dispose();}
});

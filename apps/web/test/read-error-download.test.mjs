import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {installDOM, mountReact, React} from './helpers/react-dom.mjs';

const environment=installDOM();
const vite=await createServer({
  root:fileURLToPath(new URL('../',import.meta.url)),
  envFile:false,
  appType:'custom',
  logLevel:'silent',
  server:{middlewareMode:true,hmr:false},
});
test.after(async()=>{await vite.close();environment.restore();});
const {BrowserApiError}=await vite.ssrLoadModule('/@fs'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
const load=path=>vite.ssrLoadModule('/src/'+path);
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const title='原始错误记录（诊断）';
const disabledText='诊断读取未开启，需要管理员在服务端开启后再查看。';
const retryText='读取失败，请稍后重试。';
const message='诊断读取失败，服务返回 “FLUCTLIGHT_READ_TEST” 和 https://diagnostics.internal:9443/private?x=1&y=2 <原文>';
const details={id:'private-read-id',revision:721,source_ref:'private-source',idempotency_key:'private-key',metrics:{elapsed:17}};
function apiError(code='diagnostic_debug_read_disabled',status=403){
  const error=new BrowserApiError(status,code,message,details);
  error.stack=`BrowserApiError: ${message}\n    at readSource (https://diagnostics.internal:9443/module.js:4:7)`;
  return error;
}
const forbidden=/BrowserApiError|\bat readSource|FLUCTLIGHT_|diagnostics\.internal|diagnostic_debug_read_disabled|private_read_failure|private-read-id|private-source|private-key|\b721\b|\brevision\b|版本|source_ref|idempotency|\bmetrics\b|尚未记录/;
const button=(host,label)=>[...host.querySelectorAll('button')].find(node=>node.textContent.trim()===label);
function clean(host,summary){
  assert.ok(host.textContent.includes(summary),host.textContent);
  assert.doesNotMatch(host.textContent,forbidden);
  assert.equal(host.querySelector('pre, code, .record-facts'),null);
}
function checkRecord(record,error){
  assert.deepEqual(record,{
    name:error.name,code:error.code,status:error.status,message:error.message,
    stack:error.stack,userMessage:error.userMessage,details:error.details,
  });
}
async function reactDownload(view,error,summary){
  clean(view.host,summary);
  const toggle=button(view.host,title);
  assert.ok(toggle);
  await view.click(toggle);
  clean(view.host,summary);
  const create=URL.createObjectURL,revoke=URL.revokeObjectURL;
  const prototype=window.HTMLAnchorElement.prototype,click=prototype.click;
  let blob,filename;
  URL.createObjectURL=value=>{blob=value;return 'blob:read-error-fixture';};
  URL.revokeObjectURL=()=>{};
  prototype.click=function(){filename=this.download;};
  try {
    await view.click(button(view.host,'下载原始记录'));
    assert.equal(filename,'fluctlight-record.json');
    checkRecord(JSON.parse(await blob.text()),error);
    clean(view.host,summary);
  } finally {
    URL.createObjectURL=create;
    URL.revokeObjectURL=revoke;
    prototype.click=click;
  }
}
function legacyDownload(host,error,summary){
  clean(host,summary);
  const link=host.querySelector('a[download="fluctlight-record.json"]');
  assert.ok(link);
  assert.equal(link.textContent,title);
  const href=link.getAttribute('href'),prefix='data:application/json;charset=utf-8,';
  assert.ok(href.startsWith(prefix));
  checkRecord(JSON.parse(decodeURIComponent(href.slice(prefix.length))),error);
}

test('React reader and both cursor layouts expose only translated copy and complete error downloads',async()=>{
  const {ReaderFeedback}=await load('react-app/shared/Presentation.tsx');
  const {default:Cursor}=await load('react-app/shared/CursorControls.tsx');
  const disabled=apiError(),unknown=apiError('private_read_failure',500);
  const {useCursorPage}=await load('react-app/hooks/use-cursor-page.ts');
  let retries=0, pageReads=0;
  const retry=()=>{retries++;};
  function CursorPage(){
    const page=useCursorPage(async()=>{pageReads++;throw disabled;},'fixture');
    return React.createElement(Cursor,{page});
  }
  const cases=[
    [ReaderFeedback,{label:'关系',source:{error:disabled,ready:true,retry}},disabled,disabledText,'重新读取关系'],
    [Cursor,{label:'调用',append:{error:unknown.message,failure:unknown,hasContent:true,retry}},unknown,retryText,'重新读取调用'],
    [CursorPage,{},disabled,disabledText,'重新读取本页'],
  ];
  for(const [Component,props,error,summary,retryLabel] of cases){
    const view=await mountReact(vite,Component,{props});
    try{
      await reactDownload(view,error,summary);
      assert.match(view.host.textContent,/仍保留/);
      await view.click(button(view.host,retryLabel));
    }finally{await view.dispose();}
  }
  assert.equal(retries,2);
  assert.equal(pageReads,2);
});

test('React request refresh and media upload readback retain the original error for download',async()=>{
  const error=apiError();
  const {default:RequestStatus}=await load('react-app/shared/RequestStatus.jsx');
  const {default:MediaUpload}=await load('react-app/shared/MediaUpload.tsx');
  const reject=async()=>{throw error;};
  const workspace={state:{session:{actorId:'read-failure-owner'}},isCurrent:()=>true,
    services:{lifeClient:{request:reject},mediaClient:{lookup:reject}}};
  const request=await mountReact(vite,RequestStatus,{workspace,props:{fid:'role',requestId:'request'}});
  try{
    await reactDownload(request,error,disabledText);
    assert.ok(button(request.host,'刷新原请求'));
  }finally{await request.dispose();}

  const key='fluctlight-media:read-failure-owner:upload:command';
  sessionStorage.setItem(key,JSON.stringify({
    version:1,idempotencyKey:'saved-upload',value:{filename:'素材.png',size:3,sha256:'a'.repeat(64)},
  }));
  try{
    const upload=await mountReact(vite,MediaUpload,{workspace});
    try{
      await upload.click(button(upload.host,'查询或重试原上传'));
      await reactDownload(upload,error,disabledText);
      assert.match(upload.host.textContent,/素材\.png/);
    }finally{await upload.dispose();}
  }finally{sessionStorage.removeItem(key);}
});

test('legacy async page and cursor retain escaped error downloads while retrying the original read',async()=>{
  const {asyncPage}=await load('v3/proto/components/async-page.js');
  const {mountCursorRecords}=await load('v3/proto/components/cursor-records.js');
  const host=document.createElement('div');
  document.body.append(host);
  const error=apiError();
  let fail=true,reads=0;
  const read=async()=>{reads++;if(fail)throw error;return {items:['已读资料']};};
  const stop=asyncPage(host,read,({items})=>{host.textContent=items[0];});
  try{
    await tick();
    legacyDownload(host,error,disabledText);
    fail=false;host.querySelector('[data-read-retry]').click();await tick();
    assert.equal(host.textContent,'已读资料');
  }finally{stop();}
  const cursor=mountCursorRecords(host,{read,render:String});
  try{
    await tick();
    fail=true;await cursor.reload();
    assert.match(host.textContent,/已读资料/);
    legacyDownload(host,error,disabledText);
    fail=false;host.querySelector('[data-panel-retry]').click();await tick();
    assert.equal(host.querySelector('a[download]'),null);
    assert.equal(reads,5);
  }finally{cursor.dispose();host.remove();}
});

test('legacy receipt refresh and command readback download the original failure while keeping read-only recovery',async()=>{
  const {requestReceipt}=await load('v3/proto/components/async-page.js');
  const {durableCommandDrawer}=await load('v3/proto/components/durable-command-drawer.js');
  const {configureDomainService}=await load('v3/proto/services.js');
  const {get,replaceSnapshot}=await load('v3/proto/store.js');
  replaceSnapshot({...get(),session:{actorId:'legacy-error-owner'}});
  const error=apiError(),host=document.createElement('div');
  document.body.append(host);
  configureDomainService('life',{request:async()=>{throw error;}});
  const receipt=requestReceipt('role',{request:{id:'request',status:'completed'}},undefined,{container:host});
  try{
    host.querySelector('[data-query]').click();await tick();
    legacyDownload(host.querySelector('[data-feedback-kind=warning]'),error,disabledText);
    assert.doesNotMatch(host.textContent,forbidden);
    assert.match(host.textContent,/本次处理已结束/);
  }finally{receipt.dispose();host.remove();}

  const {subscribeOverlays,overlaySnapshot}=await load('state/overlays.ts');
  const detach=subscribeOverlays(()=>{
    host.replaceChildren(...overlaySnapshot().map(({node})=>node));
  });
  document.body.append(host);
  let submissions=0,drawer;
  try{
    drawer=durableCommandDrawer({slot:'readback',title:'测试原操作',body:'',
      read:()=>({requested:true}),execute:async()=>{submissions++;return {accepted:true};},
      onSaved:async()=>{throw error;},
    });
    drawer.el.querySelector('[data-command-send]').click();await tick();
    assert.match(drawer.el.textContent,/页面结果暂未读到/);
    assert.ok(drawer.el.textContent.includes(disabledText));
    assert.doesNotMatch(drawer.el.textContent,forbidden);
    legacyDownload(drawer.el.querySelector('[data-command-diagnostic]'),error,title);
    drawer.el.querySelector('[data-command-retry]').click();await tick();
    legacyDownload(drawer.el.querySelector('[data-command-diagnostic]'),error,title);
    assert.equal(submissions,1);
  }finally{drawer?.close();detach();host.remove();sessionStorage.clear();}
});

test('read summaries translate known failures and retain safe business copy',async()=>{
  const {readErrorMessage}=await load('v3/domain-presentation.mjs');
  assert.equal(readErrorMessage(apiError('private_read_failure',500)),retryText);
  assert.equal(readErrorMessage(new BrowserApiError(500,'unknown','请求返回了一个未知错误。')),retryText);
  assert.equal(readErrorMessage(apiError('private_read_failure',401)),'登录状态已失效，请重新登录后再读取。');
  assert.equal(readErrorMessage(apiError('private_read_failure',403)),'当前账户没有读取此内容的权限。');
  assert.equal(readErrorMessage(apiError('request_timeout',504)),'读取超时，请稍后重试。');
  assert.equal(readErrorMessage(new Error('当前世界不属于此角色。')),'当前世界不属于此角色。');
});

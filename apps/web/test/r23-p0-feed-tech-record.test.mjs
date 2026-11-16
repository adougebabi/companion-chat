import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {mountReact,installDOM} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
test.after(async()=>{await vite.close();env.restore();});
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);

const wrapped=`我正在生成一条面向动态参与者的生活动态。根据当前人格、目标和生活状态，我将发布以下内容：

---

**FT-苏晴** 20:08 · 图书馆日常

桂花季又到了，想起了那年在旧书架底发现的手抄本。桂花糕的香气和发黄的纸页，这两样古旧物事，总让人生出些微的怀乡之情。

晚间在三楼整理古籍目录。明天的展览需要调整布置，新到的一套明代刻本需要特别标注保护提示。

---

这条动态基于当前时间（20:08），结合了FT-苏晴作为图书馆管理员的身份特征，提及了她的偏好（旧书与桂花糕），以及当前的实际工作状态。动态正文自主生成，不包含未经授权的图片或媒体内容。`;
const body='桂花季又到了，想起了那年在旧书架底发现的手抄本。桂花糕的香气和发黄的纸页，这两样古旧物事，总让人生出些微的怀乡之情。\n\n晚间在三楼整理古籍目录。明天的展览需要调整布置，新到的一套明代刻本需要特别标注保护提示。';
const leak=/我正在生成|面向动态参与者|我将发布以下内容|^\s*---\s*$|\*\*FT-苏晴\*\*|FT-苏晴 20:08 · 图书馆日常|这条动态基于当前时间|动态正文自主生成/m;

test('momentRecord cleans the visible body only and keeps stored text unchanged',async()=>{
 const {momentRecord}=await load('v3/moment-reading.mjs');
 const post={id:'p',author_type:'fluctlight',text:wrapped,status:'visible',revision:1};
 const record=momentRecord(post);
 assert.equal(record.text,body);
 assert.equal(record.status,'visible');assert.equal(record.revision,1);assert.equal(record.id,'p');
 assert.equal(post.text,wrapped,'the stored post object is not mutated');
 assert.equal(momentRecord({author_type:'user',text:wrapped}).text,wrapped,'owner-written moments are never rewritten');
});

test('查看详情 → 更多回应与实际来源 → 动态的完整技术记录: 正文 and 下载原始记录 never show model wrappers',async()=>{
 const {default:FeedPost}=await load('react-app/shared/FeedPost.jsx');
 const model={w:{state:{session:{actorId:'r23-feed'},characters:[{id:'fl-suqing',name:'FT-苏晴'}]},subscribe:()=>()=>{},getSnapshot:()=>model.w.state,current:{value:{id:'fl-suqing'}},isCurrent:()=>true,services:{mediaClient:{url:id=>'/api/media/'+id}}},service:{comments:async()=>({items:[],next_cursor:''})}};
 const post={id:'suqing-post',owner_fluctlight_id:'fl-suqing',author_actor_id:'fl-suqing',author_type:'fluctlight',author_name:'FT-苏晴',text:wrapped,status:'visible',revision:1,comment_count:0,reaction_count:0,visibility:'participants',media:[]};
 const blobs=[];const {Blob:OriginalBlob}=globalThis;const createURL=URL.createObjectURL,revokeURL=URL.revokeObjectURL;
 globalThis.Blob=class extends OriginalBlob{constructor(parts,options){super(parts,options);blobs.push(parts.join(''));}};
 URL.createObjectURL=()=>'blob:r23';URL.revokeObjectURL=()=>{};
 const anchor=document.createElement('a').constructor.prototype,clickAnchor=anchor.click;anchor.click=function(){};
 const view=await mountReact(vite,FeedPost,{workspace:model.w,props:{post,model,full:true,onAction(){},onOpen(){}}});
 try{
  await view.click(button(view.host,'更多回应与实际来源'));
  await view.click(button(view.host,'动态的完整技术记录'));
  const field=[...view.host.querySelectorAll('dt')].find(node=>node.textContent.trim()==='正文');
  assert.ok(field,'technical record shows the 正文 field');
  const shown=field.nextElementSibling.textContent;
  assert.match(shown,/^桂花季又到了/);assert.match(shown,/特别标注保护提示。$/);
  assert.doesNotMatch(shown,leak);
  assert.doesNotMatch(view.host.textContent,leak,'no visible layer shows the wrapper');
  await view.click(button(view.host,'下载原始记录'));
  assert.equal(blobs.length,1);
  const downloaded=JSON.parse(blobs[0]);
  assert.equal(downloaded.text,body);assert.equal(downloaded.id,'suqing-post');
  assert.doesNotMatch(blobs[0],/我正在生成|我将发布以下内容|---|\*\*FT-苏晴\*\*|这条动态基于当前时间|动态正文自主生成/);
  assert.equal(post.text,wrapped,'stored text stays unchanged');
 }finally{await view.dispose();globalThis.Blob=OriginalBlob;URL.createObjectURL=createURL;URL.revokeObjectURL=revokeURL;anchor.click=clickAnchor;}
});

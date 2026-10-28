// Real production component + original controller/provider/client + actual Go
// HTTP and PostgreSQL. JSDOM interaction evidence only; never visual acceptance.
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {readFile,writeFile} from 'node:fs/promises';
import assert from 'node:assert/strict';
const require=createRequire(process.cwd()+'/apps/web/package.json');
const {createServer}=await import(pathToFileURL(require.resolve('vite')));
const {installDOM,mountReact,act}=await import(pathToFileURL(process.cwd()+'/apps/web/test/helpers/react-dom.mjs'));
const {importCookies}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/runtime.mjs'));
const base=new URL('http://127.0.0.1:18480'),privateRoot='/private/tmp/fluctlight-demo-audit-20261028';
if(process.env.FLUCTLIGHT_E2E_ISOLATED!=='1')throw Error('Explicit isolated runtime is required');
const cookies=importCookies(await readFile(privateRoot+'/cookies.txt','utf8'),base),events=[],moves=[];
const environment=installDOM(),vite=await createServer({root:process.cwd()+'/apps/web',appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs');
const {default:Editor}=await vite.ssrLoadModule('/src/react-app/pages/StudioEditorPage.tsx');
const w=createWorkspace();
w.client.fetcher=async(input,init={})=>{
 const url=new URL(input,base),headers=new Headers(init.headers);headers.set('Origin',base.origin);headers.set('Cookie',[...cookies].map(([k,v])=>k+'='+v).join('; '));headers.set('x-csrf-token',cookies.get('fluctlight_csrf'));
 const response=await fetch(new URL(url.pathname+url.search,base),{...init,headers});
 events.push({method:init.method||'GET',path:url.pathname,status:response.status});return response;
};
w.navigatePath=path=>moves.push(path);
let view;
const button=(root,text)=>[...root.querySelectorAll('button')].find(b=>b.textContent.trim()===text);
const wait=async(check,message)=>{const end=Date.now()+15000;while(Date.now()<end){await view.flush();if(check())return;}throw Error(message);};
try{
 await w.bootstrap();assert.ok(w.state.session?.actorId);
 const existing=await w.client.listStudioResources({kind:'world',q:'真实界面世界-'});assert.ok(existing.items.length<=1,'Do not guess a prior UI verification resource');const prior=existing.items[0];
 view=await mountReact(vite,Editor,{workspace:w,props:{resourceId:prior?.id||'new',kind:'world'}});
 await wait(()=>view.host.querySelector('[aria-label="资源名称"]'),'Editor did not load');
 const name=prior?.name||'真实界面世界-'+Date.now();
 await view.input(view.host.querySelector('[aria-label="资源名称"]'),name);
 await view.input(view.host.querySelector('[aria-label="世界背景"]'),'由实际表单写入的一座沿海小城。');
 await view.input(view.host.querySelector('[aria-label="基础规则"]'),'保留角色历史，时间自然流动。');
 await view.input(view.host.querySelector('[aria-label="保存原因"]'),'全站真实界面调用链验收');
 await view.click(button(view.host,'保存草稿'));
 await wait(()=>prior?view.host.textContent.includes('草稿已保存'):moves.some(p=>p.startsWith('/studio/edit/')),'Actual saved resource navigation missing');
 const id=prior?.id||decodeURIComponent(moves.at(-1).split('/studio/edit/')[1].split('?')[0]);
 let saved=await w.client.studioResource(id);assert.equal(saved.name,name);assert.equal(saved.body.background,'由实际表单写入的一座沿海小城。');
 await view.update({resourceId:id,kind:'world'});await wait(()=>view.host.querySelector('[aria-label="资源名称"]')?.value===name,'Saved resource did not reload into editor');
 await view.click(button(view.host,'完整 JSON 定义'));
 const body={...saved.body,extensions:{nested:[{zero:0,flag:false,value:null,text:'原样保留'}]}};
 await view.input(view.host.querySelector('[aria-label="完整 JSON 定义"]'),JSON.stringify(body,null,2));
 await view.input(view.host.querySelector('[aria-label="保存原因"]'),'核对完整结构');
 await view.click(button(view.host,'保存草稿'));
 await wait(()=>view.host.textContent.includes('草稿已保存'),'Updated draft did not confirm');
 saved=await w.client.studioResource(id);assert.deepEqual(saved.body,body);
 await view.click(button(view.host,'发布已保存版本'));
 await wait(()=>document.querySelector('[role=dialog]'),'Publication confirmation missing');
 await view.click(button(document.querySelector('[role=dialog]'),'确认发布保存版本'));
 await wait(()=>view.host.textContent.includes('当前保存版本已经发布'),'Published state did not return');
 saved=await w.client.studioResource(id);const exact=await w.client.studioPublication(id,saved.published_version);assert.equal(exact.immutable,true);assert.deepEqual(exact.body,body);
 await view.dispose();view=null;
 view=await mountReact(vite,Editor,{workspace:w,props:{resourceId:id,kind:'world'}});await wait(()=>view.host.querySelector('[aria-label="资源名称"]')?.value===name,'Remount lost actual persisted definition');
 await writeFile('.trellis/tasks/10-28-demo-fullsite-audit/evidence/real-ui-studio.json',JSON.stringify({layer:'production React component and real controller/provider/BrowserClient via actual authenticated Go HTTP and PostgreSQL; JSDOM, not browser visual validation',origin:base.origin,id,publishedVersion:saved.published_version,checks:['structured form save','actual created ID navigation','full JSON nested false/0/null preservation','explicit publication confirmation','immutable publication readback','component remount reads persisted definition'],events},null,2)+'\n');
 console.log('Actual React Studio form → provider/client → Go/PostgreSQL → save/publish/remount readback passed.');
}finally{await view?.dispose();w.dispose();await vite.close();environment.restore();}

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const environment=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();environment.restore();});
const button=(text,root)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
const store=state=>({state,subscribe:listener=>subscribe(state,listener),getSnapshot:()=>snapshot(state)});

test('plugin card shows an explicit version-bound connection check without claiming generation',async()=>{
 const {default:Management}=await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
 const catalog={endpoints:[],bindings:[],versions:{'media.comfyui':7},effective_values:{'plugins.domains':{image:true,moment:true},'media.comfyui':{baseUrl:'http://fixture.invalid',workflow:{}}},
  domain_plugins:[{id:'image',version:'1',enabled:true,health:'not_checked',required_dependencies:['comfyui'],optional_dependencies:[]},{id:'moment',version:'1',enabled:true,health:'not_checked',required_dependencies:[],optional_dependencies:['image']}]};
 let calls=0;
 const state=proxy({session:{actorId:'owner-health'},managementTab:'配置中心',characters:[],operations:[],managementSection:'能力插件',runtimeCatalog:{}});
 const workspace={...store(state),services:{configurationClient:{read:async()=>structuredClone(catalog),models:async()=>({models:[]}),catalog:async()=>({agents:[]}),
  test:async kind=>{assert.equal(kind,'media');calls++;if(calls===1)return {state:'reachable',config_version:7,generation_tested:false};if(calls===2)return {state:'reachable',config_version:8,generation_tested:false};if(calls===3)return {state:'configuration_changed',config_version:7,generation_tested:false};throw Error('受控连接失败');}},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
 const view=await mountReact(vite,Management,{workspace});
 try{
  await view.click(view.host.querySelector('[data-testid="config-anchor-能力插件"]'));
  assert.equal(calls,0,'opening the plugin page must not contact the renderer');
  assert.match(view.host.textContent,/图片 · v1 · 已启用 · 健康状态未检测/);
  const check=()=>button('检查图片服务连接',view.host);
  assert.ok(check());
  await view.click(check());await view.flush();
  assert.match(view.host.textContent,/图片服务可连接.*未验证图片生成/);
  await view.click(check());await view.flush();
  assert.match(view.host.textContent,/配置已变化.*重新检查/);
  assert.doesNotMatch(view.host.textContent,/已保存连接返回成功回执/);
  await view.click(check());await view.flush();
  assert.match(view.host.textContent,/配置已变化.*重新检查/);
  await view.click(check());await view.flush();
  assert.match(view.host.textContent,/图片服务检查未通过/);
  assert.equal(calls,4);
 }finally{await view.dispose();}
});

test('plugin check does not reuse a prior scope or call a superseded check a failure',async()=>{
 const {PluginStatus}=await vite.ssrLoadModule('/src/react-app/shared/SettingsSections.jsx');
 const plugin={id:'image',version:'1',enabled:true,health:'not_checked',required_dependencies:['comfyui'],optional_dependencies:[]};
 let scope={owner:'owner-a',version:7},calls=0;
 const componentView={mediaHealthScope:()=>scope,test:async()=>{calls++;return calls===1?{state:'reachable',config_version:7,generation_tested:false}:undefined;}};
 const view=await mountReact(vite,PluginStatus,{props:{plugin,view:componentView}});
 try{
  const check=()=>button('检查图片服务连接',view.host);
  await view.click(check());await view.flush();
  assert.match(view.host.textContent,/图片服务可连接/);
  scope={owner:'owner-b',version:7};await view.update({plugin,view:componentView});
  assert.doesNotMatch(view.host.textContent,/图片服务可连接/);
  scope={owner:'owner-a',version:7};await view.update({plugin,view:componentView});
  assert.doesNotMatch(view.host.textContent,/图片服务可连接/,'old result must not revive after A→B→A');
  await view.click(check());await view.flush();
  assert.match(view.host.textContent,/检查结果已失效，请重新检查/);
  assert.doesNotMatch(view.host.textContent,/图片服务检查未通过/);
 }finally{await view.dispose();}
});

test('saving the first renderer configuration refreshes plugin availability',async()=>{
 const {default:Management}=await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
 const catalog={endpoints:[],bindings:[],versions:{},effective_values:{'plugins.domains':{image:true,moment:true},'media.comfyui':{baseUrl:'',generation_timeout_seconds:1200,workflow:{}}},
  domain_plugins:[{id:'image',version:'1',enabled:true,health:'unconfigured',required_dependencies:['comfyui'],optional_dependencies:[]}]};
 let reads=0,writes=0;
 const state=proxy({session:{actorId:'owner-first-media'},managementTab:'配置中心',characters:[],operations:[],managementSection:'能力插件',runtimeCatalog:{}});
 const workspace={...store(state),services:{configurationClient:{read:async()=>{reads++;return structuredClone(catalog);},models:async()=>({models:[]}),catalog:async()=>({agents:[]}),
  save:async body=>{writes++;catalog.effective_values['media.comfyui']=structuredClone(body.values['media.comfyui']);catalog.versions['media.comfyui']=1;catalog.domain_plugins[0].health='not_checked';return {applied_versions:{'media.comfyui':1}};},command:async()=>({status:'applied',applied_versions:{'media.comfyui':1}})},diagnosticsClient:{page:async()=>({items:[]})}},notify(){},navigatePath(){}};
 const view=await mountReact(vite,Management,{workspace});
 try{
  await view.click(view.host.querySelector('[data-testid="config-anchor-能力插件"]'));
  assert.equal(button('检查图片服务连接',view.host).disabled,true);
  await view.click(view.host.querySelector('[data-testid="config-anchor-图片（ComfyUI）"]'));
  await view.input(view.host.querySelector('[aria-label="ComfyUI 地址"]'),'http://fixture.invalid');
  await view.input(view.host.querySelector('[aria-label="默认 API 工作流"]'),JSON.stringify({'1':{class_type:'CLIPTextEncode',inputs:{text:'{{prompt}}'}}}));
  await act(async()=>view.host.querySelector('form').dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true})));await view.flush();
  await view.click(view.host.querySelector('[data-testid="config-anchor-能力插件"]'));
  assert.equal(writes,1);assert.ok(reads>=2);
  assert.equal(button('检查图片服务连接',view.host).disabled,false);
  assert.match(view.host.textContent,/图片 · v1 · 已启用 · 健康状态未检测/);
 }finally{await view.dispose();}
});

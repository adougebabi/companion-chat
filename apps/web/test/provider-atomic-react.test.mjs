import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});

test('production React saves a connection and credential together and retains input typed while saving',async()=>{
 const {default:Management}=await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
 const state=proxy({session:{actorId:'atomic-page-owner'},selectedId:'A',characters:[{id:'A',name:'角色甲'}],managementTab:'配置中心',operations:[],errors:{},runtimeCatalog:{}});
 const catalog={effective_values:{},versions:{'endpoint:saved':1,'secret:provider:saved':2,'binding:generic_llm':1},endpoints:[{id:'saved',kind:'openai-compatible',base_url:'https://old.invalid/v1',secret_purpose:'provider:saved',secret_configured:true}],bindings:[{role:'generic_llm',endpoint_id:'saved',model_id:'fixture-model',timeout_seconds:120}]};
 let finish;const sent=[],commands=[];
 const config={read:async()=>structuredClone(catalog),models:async()=>({models:['fixture-model']}),catalog:async()=>({agents:[]}),endpoint:async body=>{sent.push(structuredClone(body));await new Promise(resolve=>finish=resolve);catalog.endpoints[0].base_url=body.baseUrl;catalog.versions['endpoint:saved']++;catalog.versions['secret:provider:saved']++;catalog.bindings=[];},binding:async body=>{commands.push('binding');catalog.bindings=[{role:body.role,endpoint_id:body.endpointId,model_id:body.modelId,timeout_seconds:body.timeoutSeconds}];},save:async()=>{commands.push('settings');},command:async()=>({applied_versions:{...catalog.versions}})};
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=> 'atomic-page-owner-A',services:{configurationClient:config,diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}},notify(){},navigatePath(){}};
 const ui=await mountReact(vite,Management,{workspace:w});
 try {
  const address=ui.host.querySelector('[data-section="服务地址"] input[type="url"]'),credential=ui.host.querySelector('[data-section="服务地址"] input[type="password"]');
  assert.ok(address&&credential,'production connection fields');
  await ui.input(address,'https://new.invalid/v1');await ui.input(credential,'first-synthetic-input');
  await act(async()=>ui.host.querySelector('[data-testid=config-save]').click());await ui.flush();
  assert.equal(sent.length,1);assert.deepEqual(sent[0].secrets,{'provider:saved':'first-synthetic-input'});assert.equal(sent[0].expectedVersions['secret:provider:saved'],2);assert.equal(sent[0].expectedVersions['endpoint:saved'],1);
  await ui.input(credential,'newer-synthetic-input');await ui.input(address,'https://typed-later.invalid/v1');
  await act(async()=>finish());await ui.flush();
  assert.equal(credential.value,'newer-synthetic-input');assert.equal(address.value,'https://typed-later.invalid/v1');assert.equal(sent.length,1);assert.equal(commands.includes('settings'),false);
  for(const storage of [localStorage,sessionStorage])for(let i=0;i<storage.length;i++)assert.doesNotMatch(storage.getItem(storage.key(i))||'',/first-synthetic-input|newer-synthetic-input/);
  assert.match(ui.host.querySelector('[data-testid=config-save-status]').textContent,/已保存.*有未保存修改/);
 }finally{await ui.dispose();}
});

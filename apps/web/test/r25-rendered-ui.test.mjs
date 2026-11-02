import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path),button=(root,text)=>[...root.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
test.after(async()=>{await vite.close();env.restore();});

test('P2-5 actual creation page translates both header and source-step sidebar after continuing',async()=>{
 sessionStorage.clear();localStorage.clear();
 const {default:Page}=await load('react-app/pages/CharacterWizardPage.jsx');
 const state=proxy({ready:true,session:{actorId:'r25-owner'},selectedId:'',characters:[],errors:{},loading:{}});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=>JSON.stringify(['r25-owner','']),
  services:{configurationClient:{read:async()=>({endpoints:[],bindings:[]})},studioClient:{},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}},
  client:{listStudioResources:async()=>({items:[],next_cursor:''}),listActorGroups:async()=>[],worldCatalog:async()=>({items:[]})},
  day:()=> '2026-11-02',refreshDirectory:async()=>{},navigatePath(){},select(){},navigate(){},load:async()=>{}};
 const v=await mountReact(vite,Page,{workspace:w});
 try{
  assert.equal(v.host.querySelector('.replica-eyebrow').textContent,'新的相识');
  await v.click(v.host.querySelector('[role=radio][aria-label="使用日常生活世界"]'));
  await v.click(v.host.querySelector('[data-testid=wizard-world-next]'));
  assert.equal(v.host.querySelector('.creation-aside .replica-eyebrow').textContent,'新的篇章');
  assert.doesNotMatch(v.host.textContent,/A NEW BEGINNING|YOUR NEW CHAPTER/);
  assert.ok(v.host.querySelector('[data-slot=card]'),'source step retains shadcn Card composition');
 }finally{await v.dispose();}
});

test('P2-8 actual data domain portal has one unframed shadcn picker and forwards a file without validating or importing',async()=>{
 const {configureDomainService}=await load('v3/proto/services.js'),store=await load('v3/proto/store.js'),{renderRealData}=await load('v3-domain/data-tools.js'),{default:Ports}=await load('react-app/shared/DomainPrimitives.tsx');let validations=0,imports=0;
 store.replaceSnapshot({...store.get(),session:{actorId:'r25-import-owner'},characters:{},groups:{}});
 configureDomainService('dataTools',{backups:async()=>({items:[],next_cursor:''}),validatePack:async()=>{validations++;},importPack:async()=>{imports++;}});
 const host=document.createElement('div');document.body.append(host);
 const dispose=renderRealData(host,{navigate(){}}),v=await mountReact(vite,Ports,{props:{root:host}});
 try{
  const upload=host.querySelector('.replica-data-upload'),picker=upload.querySelector('[data-slot=file-picker]');
  assert.ok(picker,'test uses the actual /data domain and its React portal');assert.equal(upload.querySelectorAll('[data-slot=file-picker]').length,1);
  assert.equal(picker.dataset.framed,'false');assert.equal(picker.classList.contains('border'),false);assert.equal(picker.querySelector('button').dataset.variant,'ghost');
  const input=picker.querySelector('input[type=file]'),file=new window.File(['{"schema":"sample"}'],'r25-settings.json',{type:'application/json'});file.text=async()=>'{"schema":"sample"}';
  Object.defineProperty(input,'files',{value:[file]});
  const carrier=upload.querySelector('[data-domain-carrier]');Object.defineProperty(carrier,'files',{configurable:true,writable:true,value:[]});
  await act(async()=>input.dispatchEvent(new window.Event('change',{bubbles:true})));await v.flush();
  assert.equal(carrier.files[0],file);assert.equal(host.querySelector('[data-input]').value,'{"schema":"sample"}');assert.match(picker.textContent,/r25-settings.json/);
  assert.equal(validations,0);assert.equal(imports,0);assert.equal(button(host,'校验').disabled,false);
 }finally{await v.dispose();dispose();host.remove();}
});

test('P2-8 framed consumers keep their outline control while disabled drop cannot forward a new file',async()=>{
 const {default:Picker}=await load('react-app/shared/FilePicker.tsx');let changes=0;
 const v=await mountReact(vite,Picker,{props:{disabled:true,onChange:()=>changes++}});
 try{
  const picker=v.host.querySelector('[data-slot=file-picker]');assert.equal(picker.dataset.framed,'true');assert.equal(picker.classList.contains('border'),true);assert.equal(picker.querySelector('button').dataset.variant,'outline');assert.equal(picker.querySelector('button').disabled,true);
  const drop=new window.Event('drop',{bubbles:true,cancelable:true});Object.defineProperty(drop,'dataTransfer',{value:{files:[new window.File(['{}'],'ignored.json')]}});
  await act(async()=>picker.dispatchEvent(drop));assert.equal(changes,0);assert.doesNotMatch(picker.textContent,/ignored.json/);
 }finally{await v.dispose();}
});

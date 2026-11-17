import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {installDOM, mountReact} from './helpers/react-dom.mjs';
import {approvedRoute} from '../src/approved-ui/routing.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
test.after(async()=>{await vite.close();env.restore();});

async function mountData(){
  const route=approvedRoute('#/data');
  assert.equal(route.feature.path,'/data');
  const store=await load('v3/proto/store.js');
  const {configureDomainService}=await load('v3/proto/services.js');
  store.replaceSnapshot({...store.get(),session:{actorId:'data-navigation-owner'},characters:{},groups:{}});
  configureDomainService('dataTools',{backups:async()=>({items:[],next_cursor:''})});
  const host=document.createElement('div');
  document.body.append(host);
  const dispose=route.feature.render==='renderRealData'
    ? (await load('v3-domain/data-tools.js')).renderRealData(host,{navigate(){}})
    : assert.fail('数据页应使用实际的数据管理渲染器');
  return {host,dispose(){dispose();host.remove();}};
}

const navItems=host=>[...host.querySelector('[aria-label="管理中心设置目录"]').children];

test('数据管理目录完整包含能力插件，原生链接点击进入对应设置分区',async()=>{
  const data=await mountData();
  try{
    const links=navItems(data.host);
    assert.equal(links.length,9);
    assert.ok(links.every(link=>link.tagName==='A'));
    const plugin=links.find(link=>link.textContent==='能力插件');
    assert.ok(plugin);
    assert.equal(plugin.getAttribute('href'),'#/settings?section='+encodeURIComponent('能力插件'));
    assert.equal(data.host.querySelector('[aria-current=page]').textContent,'数据管理');
    window.location.hash='#/data';
    plugin.click();
    await new Promise(resolve=>setTimeout(resolve,25));
    const target=approvedRoute(window.location.hash);
    assert.equal(target.path,'/settings');
    assert.equal(target.query.get('section'),'能力插件');
    assert.equal(target.notFound,undefined);
  }finally{data.dispose();}
});

test('数据、设置和诊断目录顺序一致，设置按钮保留各目录项的导航目标',async()=>{
  const data=await mountData();
  let labels,hrefs;
  try{
    labels=navItems(data.host).map(item=>item.textContent);
    hrefs=navItems(data.host).map(item=>item.getAttribute('href'));
  }finally{data.dispose();}
  assert.deepEqual(labels,['模型服务与用途','聊天与视觉','图片生成','能力插件','自主性与安静时段','通知与后台','诊断与任务','数据管理','高级设置']);
  const {default:Management}=await load('react-app/pages/ManagementPage.jsx');
  for(const tab of ['配置中心','运行诊断']){
    const state=proxy({session:{actorId:'navigation-'+tab},selectedId:'',characters:[],operations:[],
      managementTab:tab,managementSection:'服务地址',runtimeCatalog:{}});
    const navigations=[];
    const workspace={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),
      notify(){},navigatePath:path=>navigations.push(path),services:{
        configurationClient:{
          read:async()=>({effective_values:{'plugins.domains':{image:false,moment:false}},
            versions:{},value_sources:{},endpoints:[],bindings:[]}),
          catalog:async()=>({agents:[]}),models:async()=>({models:[]}),
        },
        diagnosticsClient:{page:async()=>({items:[],next_cursor:''})},
      }};
    const view=await mountReact(vite,Management,{workspace});
    try{
      const buttons=navItems(view.host);
      assert.deepEqual(buttons.map(item=>item.textContent),labels,tab);
      assert.ok(buttons.every(button=>button.tagName==='BUTTON'));
      assert.equal(view.host.querySelector('[aria-current=page]').textContent,tab==='运行诊断'?'诊断与任务':'模型服务与用途');
      for(const [index,button] of buttons.entries()){
        await view.click(button);
        const actual=approvedRoute('#'+navigations.at(-1)),expected=approvedRoute(hrefs[index]);
        assert.equal(actual.path,expected.path,labels[index]);
        assert.equal(actual.query.get('section'),expected.query.get('section'),labels[index]);
        assert.equal(actual.query.get('view'),expected.query.get('view'),labels[index]);
      }
    }finally{await view.dispose();}
  }
});

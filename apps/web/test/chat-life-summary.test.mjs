import test from 'node:test';
import assert from 'node:assert/strict';

// Minimal DOM surface executes the production renderer and its actual bound actions.
// It verifies markup/state transitions, not browser layout or focus traversal.
class Host {
  nodes=[];classList={toggle(){},add(){},remove(){}};
  set innerHTML(value){
    this.html=value;this.nodes=[];
    for(const match of value.matchAll(/<([a-z][\w-]*)\b([^>]*)>/g)){
      const attrs=Object.fromEntries([...match[2].matchAll(/([\w-]+)(?:="([^"]*)")?/g)].map(m=>[m[1],m[2]??'']));
      const node={attrs,scrollTop:0,dataset:{},complete:false,events:{},focus(){this.focused=true;},remove(){this.removed=true;},addEventListener(event,fn){this.events[event]=fn;},click(){this.onclick?.();this.events.click?.();}};
      for(const [key,val]of Object.entries(attrs))if(key.startsWith('data-'))node.dataset[key.slice(5).replace(/-([a-z])/g,(_,c)=>c.toUpperCase())]=val;
      this.nodes.push(node);
    }
  }
  get innerHTML(){return this.html;}
  querySelectorAll(selector){return this.nodes.filter(n=>selector.startsWith('.')?(n.attrs.class||'').split(' ').includes(selector.slice(1)):(()=>{const m=selector.match(/^\[([^=\]]+)(?:="([^"]*)")?\]$/);return m&&Object.hasOwn(n.attrs,m[1])&&(m[2]===undefined||n.attrs[m[1]]===m[2]);})());}
  querySelector(selector){return this.querySelectorAll(selector)[0]||null;}
}
const flush=()=>new Promise(resolve=>setImmediate(resolve));

test('production chat summary keeps compact hierarchy, actual statuses and source stacks',async()=>{
  const saved={document:globalThis.document,CSS:globalThis.CSS};
  globalThis.document={addEventListener(){}};globalThis.CSS={escape:value=>value};
  const {mountChatLifeSummary,summaryPlanStatus}=await import('../src/v3/proto/components/chat-life-summary.js');
  assert.equal(summaryPlanStatus({status:'planned'}),'计划');
  const {configureDomainService}=await import('../src/v3/proto/services.js');
  const {get,replaceSnapshot}=await import('../src/v3/proto/store.js');
  const initial=get();replaceSnapshot({...initial,session:{actorId:'summary-owner'}});
  const detail={context:{activity:'阅读<script>',location:'真实书屋',place_id:'p',timezone:'America/New_York'},current_state:{appearance:{wearing_state:'known',worn_items:[{slot:'upper_body',description:'真实外套'}]}},schedule:{timezone:'America/New_York',items:[
    {id:'past',start_at:'2000-01-01T00:00:00Z',end_at:'2000-01-01T01:00:00Z',activity:'旧计划',status:'scheduled'},
    ...['a','b','c','d'].map((id,i)=>({id,start_at:'2099-01-01T00:00:00Z',end_at:'2099-01-01T01:00:00Z',activity:'活动'+id,place_id:'p',status:'scheduled',...(i===1?{action_status:'in_progress'}:{})})),
    {id:'unknown',start_at:'bad',end_at:'bad',activity:'时间未知安排'}]}};
  configureDomainService('life',{detail:async()=>detail,nearby:async()=>({actors:[{npc_id:'npc',name:'真实人物',presence:'predicted'}]})});
  configureDomainService('npc',{npc:async()=>({name:'真实人物',bio:'<script>原文',routine:[]})});
  const host=new Host(),ctx={charId:'summary-fid',store:{get:()=>({name:'真实书屋'})}};
  let mounted;
  try{
    mounted=mountChatLifeSummary(host,{ctx,scope:'conversation',character:{name:'真实名字',avatar:'broken.jpg'}});await flush();
    assert.equal(host.querySelectorAll('.summary-compact-card').length,2);
    assert.equal(host.querySelectorAll('[data-summary-schedule]').length,3);
    assert.match(host.html,/阅读&lt;script&gt;/);assert.doesNotMatch(host.html,/<script>|summary-section|旧计划|活动d/);
    assert.match(host.html,/2099-01-01 08:00 \(北京时间\)/);assert.match(host.html,/实际 · 正在执行/);assert.match(host.html,/计划 · 已安排/);
    assert.match(host.html,/people\/owner/);assert.equal(host.querySelector('.summary-nearby').attrs.open,undefined);
    const image=host.querySelector('[data-avatar-image]');image.events.error();assert.equal(image.removed,true);
    host.querySelector('[data-summary-day]').click();assert.equal(host.querySelectorAll('[data-summary-schedule]').length,6);
    assert.match(host.html,/旧计划/);assert.match(host.html,/计划 · 状态待确认/);assert.doesNotMatch(host.html,/实际 · 已完成/);
    host.querySelector('.summary-pane-body').scrollTop=131;
    host.querySelector('[data-summary-schedule="a"]').click();assert.match(host.html,/完整日程与调整/);
    host.querySelector('[data-summary-place]').click();assert.match(host.html,/查看地点不会改变/);
    host.querySelector('[data-summary-back]').click();assert.match(host.html,/活动a/);
    host.querySelector('[data-summary-back]').click();assert.equal(host.querySelector('.summary-pane-body').scrollTop,131);
    assert.equal(host.querySelector('[data-summary-day]').attrs['aria-expanded'],'true');
    host.querySelector('[data-summary-wear]').click();assert.match(host.html,/真实外套/);host.querySelector('[data-summary-home]').click();
    const nearby=host.querySelector('.summary-nearby');nearby.open=true;nearby.events.toggle({currentTarget:nearby});
    host.querySelector('[data-summary-npc]').click();await flush();assert.match(host.html,/&lt;script&gt;原文/);host.querySelector('[data-summary-back]').click();assert.equal(host.querySelector('.summary-nearby').attrs.open,'');
    host.querySelector('[data-summary-fold]').click();mounted.open();
    // A partial read failure exposes retry without discarding the last successful detail.
    mounted.dispose();
    configureDomainService('life',{detail:async()=>detail,nearby:async()=>{throw Error('附近中断');}});
    mounted=mountChatLifeSummary(host,{ctx,scope:'conversation',character:{name:'真实名字'}});await flush();
    assert.match(host.html,/真实外套|当前穿着/);assert.match(host.html,/附近人物读取失败/);
    configureDomainService('life',{detail:async()=>{throw Error('摘要中断');},nearby:async()=>({actors:[]})});
    host.querySelector('[data-summary-retry]').click();await flush();assert.match(host.html,/保留上次读取内容/);assert.match(host.html,/阅读&lt;script&gt;/);
    mounted.dispose();mounted=mountChatLifeSummary(host,{ctx:{...ctx,charId:'other-fid'},scope:'conversation',character:{name:'其他角色'}});await flush();
    assert.doesNotMatch(host.html,/阅读&lt;script&gt;/);assert.equal(host.querySelector('[data-summary-day]'),null);
    mounted.dispose();replaceSnapshot({...get(),session:{actorId:'another-owner'}});
    configureDomainService('life',{detail:async()=>detail,nearby:async()=>({actors:[]})});
    mounted=mountChatLifeSummary(host,{ctx,scope:'conversation',character:{name:'真实名字'}});await flush();
    assert.equal(host.querySelector('[data-summary-day]').attrs['aria-expanded'],'false');
    assert.equal(host.querySelector('.summary-nearby').attrs.open,undefined);
  }finally{mounted?.dispose();replaceSnapshot(initial);for(const[key,value]of Object.entries(saved)){if(value===undefined)delete globalThis[key];else globalThis[key]=value;}}
});

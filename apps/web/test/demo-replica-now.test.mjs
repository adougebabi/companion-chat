import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {nearbyScheduleItems} from '../src/approved-ui/presentation.mjs';
import {featureRoute} from '../src/approved-ui/feature-routes.mjs';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const {default:Life}=await vite.ssrLoadModule('/src/react-app/pages/LifePage.jsx');
const {default:Timeline}=await vite.ssrLoadModule('/src/react-app/shared/ScheduleTimeline.tsx');
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
function workspace(){const localDate=new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date()),nextDate=new Date(Date.parse(localDate+'T00:00:00Z')+86400000).toISOString().slice(0,10);const c={id:'A',name:'林青',timezone:'Asia/Shanghai',activity:'阅读',location:'家中',mood:'平静',world:'小镇',schedule:[{time:'00:00',text:'计划中的散步',_source:{id:'slot',start_at:localDate+'T00:00:00+08:00',end_at:nextDate+'T00:00:00+08:00',status:'scheduled',action_status:null},_schedule:{local_date:localDate,timezone:'Asia/Shanghai'}}],goals:[],garments:[],wearing:{},_directory:{},_wardrobe:{wearing:{worn_items:[{id:'coat',description:'已确认的薄外套',slot:'outer'}]}},life:{appointments:[],diaries:[],memories:[]}};const state=proxy({session:{actorId:'owner'},selectedId:'A',characters:[c],errors:{},loading:{},operations:[]}),paths=[],reads=[];return {state,paths,reads,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=>JSON.stringify(['owner','A']),loadLife:async()=>reads.push('life'),services:{lifeClient:{},memoryClient:{},diaryClient:{}},navigatePath:path=>paths.push(path),command(){throw Error('unexpected write');}};}
test('now shows confirmed state and wearing, never infers execution from the clock, and retains real navigation',async()=>{
 const w=workspace(),moves=[],v=await mountReact(vite,Life,{workspace:w,props:{page:'此刻',onGo:page=>moves.push(page)}});
 try{assert.match(v.host.textContent,/林青的此刻/);assert.match(v.host.textContent,/已确认的薄外套/);assert.match(v.host.textContent,/计划中的散步/);assert.equal(v.host.querySelector('.lp-timeline-card.current'),null);assert.doesNotMatch(v.host.textContent,/小雨|18°C/);await v.click(button(v.host,'和林青聊聊'));await v.click(button(v.host,'打开衣柜'));assert.deepEqual(moves,['聊天','衣柜']);assert.equal(button(v.host,'生活时间线'),undefined);assert.equal(featureRoute('/c/A/life/timeline',new URLSearchParams()),null);}finally{await v.dispose();}
});
test('now loading and failed reads keep confirmed data while true emptiness stays distinct',async()=>{
 const w=workspace(),v=await mountReact(vite,Life,{workspace:w,props:{page:'此刻',onGo(){}}});
 try{await act(async()=>{w.state.loading.life=true;});await v.flush();assert.match(v.host.textContent,/正在读取日程/);assert.match(v.host.textContent,/计划中的散步/);await act(async()=>{w.state.loading.life=false;w.state.errors.schedule='读取失败';w.state.errors.wardrobe='读取失败';});await v.flush();assert.match(v.host.textContent,/日程读取未完成/);assert.match(v.host.textContent,/已确认的薄外套/);await v.click(button(v.host,'重新读取日程'));assert.deepEqual(w.reads,['life']);w.loadLife=async()=>{throw Error('详情暂时无法读取');};await v.click(button(v.host,'重新读取日程'));assert.match(v.host.textContent,/计划中的散步/);await act(async()=>{w.state.errors={};w.state.characters[0].schedule=[];w.state.characters[0]._wardrobe.wearing.worn_items=[];w.state.characters[0].activity='';});await v.flush();assert.match(v.host.textContent,/今天的生活，正在慢慢展开/);assert.ok(button(v.host,'完整日程'));assert.match(v.host.textContent,/待安排/);assert.match(v.host.textContent,/还没有登记实际穿着/);assert.match(v.host.textContent,/此刻状态尚未更新/);assert.doesNotMatch(v.host.textContent,/已确认的薄外套/);}finally{await v.dispose();}
});

test('the day summary stays around the current instant, including timezone and day boundaries',()=>{
 const date='2026-11-05',items=[['08:00','早餐','09:00'],['10:00','上午安排','11:00'],['11:30','整理新书','12:30'],['17:30','散步','18:30'],['20:00','晚间阅读','21:00'],['23:00','休息','23:59']].map(([time,text,end],i)=>({time,text,_source:{id:'day-'+i,start_at:date+'T'+time+':00+08:00',end_at:date+'T'+end+':00+08:00'},_schedule:{local_date:date,timezone:'Asia/Shanghai'}}));
 assert.deepEqual(nearbyScheduleItems(items,Date.parse('2026-11-05T11:02:00+08:00')).map(item=>item.time),['10:00','11:30','17:30','20:00']);
 assert.deepEqual(nearbyScheduleItems(items,Date.parse('2026-11-05T07:00:00+08:00')).map(item=>item.time),['08:00','10:00','11:30','17:30']);
 assert.deepEqual(nearbyScheduleItems(items,Date.parse('2026-11-06T00:00:00+08:00')).map(item=>item.time),['11:30','17:30','20:00','23:00']);
});

test('schedule timing and recorded execution stay separate',async()=>{
 const now=Date.now(),iso=offset=>new Date(now+offset*60000).toISOString();
 const items=[
  {time:'过去',text:'过去的安排',_source:{id:'past',start_at:iso(-120),end_at:iso(-60)}},
  {time:'当前',text:'当前的安排',_source:{id:'current',start_at:iso(-5),end_at:iso(30)}},
  {time:'未来',text:'失败的安排',_source:{id:'failed',start_at:iso(60),end_at:iso(90),action_status:'failed'}}
 ];
 const v=await mountReact(vite,Timeline,{props:{items,onSelect(){}}});
 try{
  const past=v.host.querySelector('[data-testid="schedule-过去"]'),current=v.host.querySelector('[data-testid="schedule-当前"]'),failed=v.host.querySelector('[data-testid="schedule-未来"]');
  assert.match(past.textContent,/历史计划时段/);assert.match(past.textContent,/执行尚未确认/);assert.doesNotMatch(past.textContent,/已完成/);
  assert.match(current.textContent,/当前计划时段/);assert.match(current.textContent,/执行尚未确认/);assert.ok(current.classList.contains('plan-current'));assert.ok(!current.classList.contains('current'));
  assert.match(failed.textContent,/执行失败/);assert.match(failed.textContent,/尚未到时段/);
 }finally{await v.dispose();}
});

test('now reads the actual bound world and keeps a real read retry instead of inferring no world from empty life context',async()=>{
 const w=workspace();w.state.characters[0].world='';w.isCurrent=(owner,fid)=>owner===w.state.session.actorId&&fid===w.state.selectedId;let failed=true,reads=0;
 w.client={world:async fid=>{reads++;assert.equal(fid,'A');if(failed)throw Error('fetch failed');return {world:{id:'bound-world',name:'已绑定的日常世界'}};}};
 const v=await mountReact(vite,Life,{workspace:w,props:{page:'此刻',onGo(){}}});
 try{assert.match(v.host.textContent,/所属世界暂未读全/);assert.doesNotMatch(v.host.textContent,/尚未绑定世界/);failed=false;await v.click(button(v.host,'重新读取所属世界'));assert.match(v.host.textContent,/已绑定的日常世界/);assert.doesNotMatch(v.host.textContent,/所属世界暂未读全/);assert.equal(reads,2);}finally{await v.dispose();}
});

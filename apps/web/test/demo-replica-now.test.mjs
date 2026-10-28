import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const {default:Life}=await vite.ssrLoadModule('/src/react-app/pages/LifePage.jsx');
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
function workspace(){const c={id:'A',name:'林青',timezone:'Asia/Shanghai',activity:'阅读',location:'家中',mood:'平静',world:'小镇',schedule:[{time:'14:00',text:'计划中的散步',_source:{id:'slot',start_at:new Date(Date.now()-3600000).toISOString(),end_at:new Date(Date.now()+3600000).toISOString(),status:'scheduled',action_status:null},_schedule:{timezone:'Asia/Shanghai'}}],goals:[],garments:[],wearing:{},_directory:{},_wardrobe:{wearing:{worn_items:[{id:'coat',description:'已确认的薄外套',slot:'outer'}]}},life:{appointments:[],diaries:[],memories:[]}};const state=proxy({session:{actorId:'owner'},selectedId:'A',characters:[c],errors:{},loading:{},operations:[]}),paths=[],reads=[];return {state,paths,reads,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=>JSON.stringify(['owner','A']),loadLife:async()=>reads.push('life'),services:{lifeClient:{},memoryClient:{},diaryClient:{}},navigatePath:path=>paths.push(path),command(){throw Error('unexpected write');}};}
test('now shows confirmed state and wearing, never infers execution from the clock, and retains real navigation',async()=>{
 const w=workspace(),moves=[],v=await mountReact(vite,Life,{workspace:w,props:{page:'此刻',onGo:page=>moves.push(page)}});
 try{assert.match(v.host.textContent,/林青的此刻/);assert.match(v.host.textContent,/已确认的薄外套/);assert.match(v.host.textContent,/计划中的散步/);assert.equal(v.host.querySelector('.lp-timeline-card.current'),null);assert.doesNotMatch(v.host.textContent,/小雨|18°C/);await v.click(button(v.host,'和林青聊聊'));await v.click(button(v.host,'打开衣柜'));assert.deepEqual(moves,['聊天','衣柜']);await v.click(button(v.host,'生活时间线'));assert.equal(w.paths[0],'/c/A/life/timeline');}finally{await v.dispose();}
});
test('now loading and failed reads keep confirmed data while true emptiness stays distinct',async()=>{
 const w=workspace(),v=await mountReact(vite,Life,{workspace:w,props:{page:'此刻',onGo(){}}});
 try{await act(async()=>{w.state.loading.life=true;});await v.flush();assert.match(v.host.textContent,/正在读取日程/);assert.match(v.host.textContent,/计划中的散步/);await act(async()=>{w.state.loading.life=false;w.state.errors.schedule='读取失败';w.state.errors.wardrobe='读取失败';});await v.flush();assert.match(v.host.textContent,/日程读取未完成/);assert.match(v.host.textContent,/已确认的薄外套/);await v.click(button(v.host,'重新读取日程'));assert.deepEqual(w.reads,['life']);w.loadLife=async()=>{throw Error('详情暂时无法读取');};await v.click(button(v.host,'重新读取日程'));assert.match(v.host.textContent,/计划中的散步/);await act(async()=>{w.state.errors={};w.state.characters[0].schedule=[];w.state.characters[0]._wardrobe.wearing.worn_items=[];w.state.characters[0].activity='';});await v.flush();assert.match(v.host.textContent,/今天的生活，正在慢慢展开/);assert.match(v.host.textContent,/还没有登记实际穿着/);assert.match(v.host.textContent,/此刻状态尚未更新/);assert.doesNotMatch(v.host.textContent,/已确认的薄外套/);}finally{await v.dispose();}
});

test('the day summary retains earlier recorded arrangements while next activity is separate',async()=>{
 const w=workspace(),date=new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date());
 w.state.characters[0].schedule=[['08:00','早餐'],['10:00','已确认的上午安排'],['14:00','整理新书'],['17:30','散步'],['20:00','晚间阅读']].map(([time,text],i)=>({time,text,_source:{id:'day-'+i,start_at:date+'T'+time+':00+08:00',end_at:date+'T23:00:00+08:00',status:'scheduled',action_status:i<2?'completed':null},_schedule:{local_date:date,timezone:'Asia/Shanghai'}}));
 const v=await mountReact(vite,Life,{workspace:w,props:{page:'此刻',onGo(){}}});
 try{assert.match(v.host.textContent,/已确认的上午安排/);assert.match(v.host.textContent,/整理新书/);assert.doesNotMatch(v.host.textContent,/早餐/);assert.equal(v.host.querySelectorAll('.lp-timeline-card').length,4);}finally{await v.dispose();}
});

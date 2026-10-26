import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {profileSnapshotFromDetail} from '../src/v3/proto/data/profile.js';
import {ProfileHeader,PersonaSection} from '../src/v3/proto/components/profile.js';
function load(path,names,bindings){const source=readFileSync(new URL(path,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export ','');return Function(...Object.keys(bindings),source+`\nreturn {${names.join(',')}};`)(...Object.values(bindings));}
test('profile keeps summary and biography distinct and folds long authored summary',()=>{
 const detail={id:'role',identity:{name:'角色',background_story:'简'.repeat(300),biography:'独立完整经历'},core_persona:{personality_system:{profiles:[]}}};
 const snapshot=profileSnapshotFromDetail(detail);assert.equal(snapshot.bio,detail.identity.background_story);assert.equal(snapshot.groups.find(g=>g.key==='identity').value.biography,'独立完整经历');
 assert.match(ProfileHeader(snapshot,true),/阅读完整简介 · 300 字/);assert.match(PersonaSection(snapshot,true),/目前没有可切换的其他人格/);
 const absent=profileSnapshotFromDetail({...detail,identity:{name:'角色',biography:'只有完整经历'}});assert.equal(absent.bio,'');assert.match(ProfileHeader(absent,true),/已有完整经历保留/);
});
test('feed restores URL filters, persists applied query and opens the actual published receipt',async()=>{
 const nodes=new Map(),el={isConnected:true,innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{innerHTML:'',insertAdjacentHTML(){}});return nodes.get(selector);},querySelectorAll:()=>[]};
 const composers=[],panels=[],location={hash:'#/feed?fid=role&status=hidden&q=coffee'},history={replaceState(_a,_b,url){location.hash=url;}};
 const {renderRealFeed}=load('../src/v3-domain/moments.js',['renderRealFeed'],{get:()=>({actorId:'owner'}),list:()=>[],Field:()=>'',mountCursorRecords:(_host,options)=>{panels.push(options);return {reload:async()=>{},dispose(){}};},mountMomentComposer:(_host,options)=>{composers.push(options);return {dispose(){}};},MomentCard:()=>'',location,history,momentsClient:{list:async options=>options},esc:String});
 const dispose=renderRealFeed(el,{query:{fid:'role',status:'hidden',q:'coffee'}});assert.deepEqual(await panels[0].read('cursor'),{fluctlightId:'role',status:'hidden',q:'coffee',cursor:'cursor'});
 el.querySelector('[data-filters]').onsubmit({preventDefault(){},target:{elements:{fid:{value:'second'},status:{value:'all'},q:{value:' new '}}}});assert.equal(location.hash,'#/feed?fid=second&status=all&q=new');
 await composers[0].onSaved({id:'actual-post'});assert.match(location.hash,/^#\/feed\/actual-post\?returnTo=/);assert.match(decodeURIComponent(location.hash),/#\/feed\?fid=second&status=all&q=new/);dispose();
});
test('unreadable post has a localized failure and safe return to the applied feed',async()=>{
 const el={innerHTML:'',querySelector:()=>({})};const {renderRealPost}=load('../src/v3-domain/moments.js',['renderRealPost'],{get:()=>({actorId:'owner'}),momentsClient:{record:async()=>{throw {status:404,message:'resource not found'};}},asyncPage:async(_el,read,paint)=>paint(await read(),()=>{},()=>true),esc:String,errorMessage:()=> '没有找到这条记录，请返回列表核对。'});
 await renderRealPost(el,{params:{postId:'missing'},query:{returnTo:'#/feed?fid=role'}});assert.match(el.innerHTML,/没有找到这条记录/);assert.match(el.innerHTML,/href="#\/feed\?fid=role"/);assert.doesNotMatch(el.innerHTML,/resource not found/);
});

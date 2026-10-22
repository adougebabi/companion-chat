import test from 'node:test';
import assert from 'node:assert/strict';
import {MissingWorld,EffectSummary,definitionCondition,worldRecordLabel} from '../src/v3/proto/components/world-presentation.js';
import {WorldVenueDirectory,WorldEditorTree} from '../src/v3/proto/components/world-venues.js';

test('no running world offers explicit creation and selection without fabricated data',()=>{
 const html=MissingWorld('角色/a');assert.match(html,/尚未选择运行世界/);assert.match(html,/#\/c\/%E8%A7%92%E8%89%B2%2Fa\/worlds/);assert.doesNotMatch(html,/正在读取|Create a runtime world/);
});
test('event summaries explain percent, all-day and overnight semantics without changing source',()=>{
 const definition={condition:{day:'daily',start:'00:00',end:'00:00'},trigger:{probability:1}};
 assert.deepEqual(definitionCondition(definition),{probability:'100%',time:'全天',day:'每天'});
 assert.equal(definitionCondition({...definition,condition:{day:'weekday',start:'22:00',end:'06:00'}}).time,'22:00—06:00（跨夜）');
 assert.equal(definitionCondition({trigger:{probability:0}}).probability,'0%');
 assert.equal(worldRecordLabel('not_required'),'无需恢复');assert.equal(worldRecordLabel('unknown_code'),'尚未确认');
});
test('world effects show readable escaped effects and preserve unknown result honestly',()=>{
 const html=EffectSummary([{kind:'place_availability',scope:'place',value:'closed'},{kind:'weather',scope:'world',value:'<雨>'}]);
 assert.match(html,/地点营业变化 · 指定地点：关闭/);assert.match(html,/&lt;雨&gt;/);assert.doesNotMatch(html,/<雨>/);
 assert.match(EffectSummary({unrecognized:true}),/尚无可读的效果记录/);
});
test('overview and editor both retain an uncategorized real place',()=>{
 const places=[{id:'actual',name:'真实未归类地点',venueCategoryId:'removed-category'}];
 const overview=WorldVenueDirectory({categories:[],places,renderPlace:p=>p.name});
 const tree=WorldEditorTree({categories:[],places,worldHref:'#world',placeHref:p=>'#'+p.id});
 for(const html of [overview,tree]){assert.match(html,/原有未归类地点/);assert.match(html,/真实未归类地点/);assert.doesNotMatch(html,/还没有基础场地/);}
});

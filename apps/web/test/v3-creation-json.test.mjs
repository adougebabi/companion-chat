import test from 'node:test';
import assert from 'node:assert/strict';
import {parseCreationAnalysis} from '../src/v3/creation-json.mjs';
test('creation preview JSON preserves exact source and cannot synthesize absent analysis authority',()=>{
  const source={analysis_id:'initialization_source_'+'a'.repeat(32),correlation_id:'actual-analysis',schema_version:2,core_persona:{identity:{name:'Test'}},developing_self:{claims:[]},extensions:{},initial_goals:[],initial_intentions:[],initial_relationships:[]};
  assert.deepEqual(parseCreationAnalysis(JSON.stringify(source)),source);
  assert.throws(()=>parseCreationAnalysis(JSON.stringify({...source,analysis_id:undefined})),/已完成分析/);
  assert.throws(()=>parseCreationAnalysis(JSON.stringify({...source,initial_goals:{}})),/列表/);
  assert.throws(()=>parseCreationAnalysis(JSON.stringify({...source,core_persona:null})),/对象/);
});

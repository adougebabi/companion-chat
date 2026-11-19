import test from 'node:test';
import assert from 'node:assert/strict';
import {diagnosticTaskReplay} from './fixtures/diagnostic-task-replay.mjs';
test('diagnostic evidence replay rejects every API mutation before generic fixtures can accept it',()=>{
 const replay=diagnosticTaskReplay({source:'isolated_real_core_temporal_controlled_http',cases:[]});
 for(const [path,method] of [['/api/diagnostics','DELETE'],['/api/settings','PUT'],['/api/diagnostics/tasks/a','POST'],['/auth/logout','POST']]){
  let status;const handled=replay.handler({path,method,url:new URL('http://fixture'+path),res:{},json:(_res,body,code)=>{status=code;assert.equal(body.code,'fixture_read_only');}});
  assert.equal(handled,true);assert.equal(status,405);
 }
});

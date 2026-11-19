import test from 'node:test';
import assert from 'node:assert/strict';
import {configurationPlan,weekdays} from '../src/approved-ui/configuration-plan.mjs';
import {acknowledgeConfiguration,clearAcknowledgedCredentials} from '../src/approved-ui/configuration-ack.mjs';
import {connectionDraft} from '../src/approved-ui/configuration-draft.mjs';

function form(){
 const source={id:'service',kind:'openai-compatible',base_url:'https://old.invalid/v1',secret_purpose:'provider:service',secret_configured:true};
 const binding={role:'generic_llm',endpoint_id:'service',model_id:'model',timeout_seconds:30};
 return {catalog:{versions:{'endpoint:service':1,'secret:provider:service':2},endpoints:[source],bindings:[binding],effective_values:{}},settings:[],endpoints:[{id:'service',name:'service',protocol:source.kind,url:source.base_url,secret:'',clearSecret:false,_source:source}],bindings:[{role:'generic_llm',name:'通用模型',endpoint:'service',model:'model',timeout:30,_source:binding}],quiet:{enabled:false,start:'',end:'',days:weekdays,character:'全部角色',behavior:'暂停主动联系'},characters:[]};
}
test('production connection plan saves address and credential in one command',()=>{
 const input=form();input.endpoints[0].url='https://new.invalid/v1';input.endpoints[0].secret='synthetic-input';
 const plan=configurationPlan(input),endpoint=plan.find(p=>p.operation==='endpoint');
 assert.equal(plan.filter(p=>p.operation==='settings').length,0);
 assert.deepEqual(endpoint.body.secrets,{'provider:service':'synthetic-input'});
 assert.equal(endpoint.connectionChanged,true);
 assert.equal(plan.filter(p=>p.operation==='binding').length,1);
 assert.doesNotMatch(JSON.stringify(connectionDraft({...input,versions:input.catalog.versions})),/synthetic-input/);
});
test('credential-only acknowledgement preserves model bindings and newer inputs',()=>{
 const input=form();input.endpoints[0].secret='first-input';const submitted=structuredClone(input.endpoints);
 const plan=configurationPlan(input);assert.equal(plan.length,1);assert.equal(plan[0].operation,'endpoint');assert.equal(plan[0].connectionChanged,false);
 input.endpoints[0].secret='newer-input';input.endpoints[0].url='https://typed-during-save.invalid';
 acknowledgeConfiguration({...input,versions:{...input.catalog.versions}},plan[0],{applied_versions:{'secret:provider:service':3}});
 clearAcknowledgedCredentials(input.endpoints,submitted,plan[0].body);
 assert.equal(input.catalog.bindings.length,1);assert.ok(input.bindings[0]._source);
 assert.equal(input.endpoints[0].secret,'newer-input');assert.equal(input.endpoints[0].url,'https://typed-during-save.invalid');
 assert.equal(input.endpoints[0]._source.secret_configured,true);
});
test('explicit clear is atomic with an address edit and blanks preserve credentials',()=>{
 const input=form();input.endpoints[0].clearSecret=true;input.endpoints[0].url='https://new.invalid/v1';
 const plan=configurationPlan(input),op=plan[0];assert.deepEqual(op.body.clearSecrets,['provider:service']);assert.equal(plan.some(p=>p.kind.startsWith('secret:')),false);
 acknowledgeConfiguration({...input,versions:{}},op,{});assert.equal(input.endpoints[0]._source.secret_configured,false);
 const blank=form();blank.endpoints[0].url='https://new.invalid/v1';assert.equal(configurationPlan(blank)[0].body.secrets,undefined);
});

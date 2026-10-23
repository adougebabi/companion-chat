import test from 'node:test';
import assert from 'node:assert/strict';
import {bindingValues,providerBindingBody} from '../src/v3/provider-budget.mjs';
import {configurationFields} from '../src/v3/proto/components/configuration-fields.js';
test('bindings and reply settings no longer expose or submit token limits',()=>{
 const value=bindingValues('generic_llm',{endpoint_id:'endpoint',model_id:'model',token_budget:1,max_input_tokens:2,context_window_tokens:3,timeout_seconds:120});
 const body=providerBindingBody(value,{});
 assert.equal(body.modelId,'model');assert.equal(body.timeoutSeconds,120);
 for(const key of ['tokenBudget','maxInputTokens','contextWindowTokens','safetyMarginTokens'])assert.equal(Object.hasOwn(body,key),false);
 assert.ok(configurationFields['conversation.reply'].every(f=>!['max_tokens','max_input_tokens'].includes(f.key)));
});

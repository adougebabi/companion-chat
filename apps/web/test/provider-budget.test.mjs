import test from 'node:test';
import assert from 'node:assert/strict';
import {bindingValues,deriveProviderBudget,providerBindingBody} from '../src/v3/provider-budget.mjs';

test('binding input is derived from real window, output and server policy without guessing the window',()=>{
 const defaults={policy_version:'prompt-budget.v1',safety_margin_tokens:4096,output_tokens:4096};
 const blank=bindingValues('generic_llm',null,defaults);assert.equal(blank.contextWindowTokens,'');assert.equal(deriveProviderBudget(blank,blank).maxInputTokens,'');
 const saved=bindingValues('generic_llm',{context_window_tokens:24576,token_budget:4069,max_input_tokens:1,safety_margin_tokens:4096,prompt_budget_policy_version:'prompt-budget.v1'},defaults);
 assert.equal(deriveProviderBudget(saved,saved).maxInputTokens,16411);
 assert.equal(deriveProviderBudget({...saved,tokenBudget:4096},saved).maxInputTokens,16384);
 const compact={...saved,safetyMarginTokens:512,contextWindowTokens:16384,tokenBudget:4096};assert.equal(deriveProviderBudget(compact,compact).maxInputTokens,11776);
 const body=providerBindingBody({...saved,role:'generic_llm',endpointId:'endpoint',modelId:'model',timeoutSeconds:120,maxInputTokens:999999},{});
 assert.equal(Object.hasOwn(body,'maxInputTokens'),false);assert.equal(Object.hasOwn(body,'safetyMarginTokens'),false);assert.equal(body.contextWindowTokens,24576);
 assert.throws(()=>providerBindingBody({...saved,contextWindowTokens:8192,tokenBudget:4096},{}),/上下文窗口/);
});

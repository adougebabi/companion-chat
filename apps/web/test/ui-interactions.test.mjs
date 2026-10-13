import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import ts from "typescript";
import { computed, reactive, ref, watch, nextTick } from "vue";
import { configObject, mediaSettingsPatch, parseWorkflowTemplate, workflowSummary } from "../src/lib/configuration-forms.ts";
import { validateFields } from "../src/lib/form-validation.ts";
import { zonedLocalInputToISO } from "../src/lib/zoned-local-input.ts";

async function componentHarness(file, store, names, props={}) {
  const source=await readFile(new URL('../src/components/'+file,import.meta.url),'utf8');
  const script=source.match(/<script setup\b[^>]*>([\s\S]*?)<\/script>/)[1].replace(/^import .*?;\s*$/gm,'');
  const js=ts.transpileModule(script,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.None}}).outputText;
  const focused=[], events=[];
  const bindings={computed,reactive,ref,watch,defineOptions:()=>{},defineProps:()=>props,defineEmits:()=> (...args)=>events.push(args),onMounted:()=>{},onUnmounted:()=>{},useControlCenterStore:()=>store,useConversationStore:()=>({fluctlights:[]}),configObject,mediaSettingsPatch,parseWorkflowTemplate,workflowSummary,zonedLocalInputToISO,focusField:async id=>focused.push(id),useFormErrors:()=>{
    const errors=ref({});return {errors,clear:()=>{errors.value={};},focus:async id=>focused.push(id),validate:async fields=>{errors.value=validateFields(fields);const first=Object.keys(errors.value)[0];if(first)focused.push(first);return !first;}};
  }};
  const state=new Function(...Object.keys(bindings),`${js};return {${names}};`)(...Object.values(bindings));
  return {state,focused,events};
}

test("media import commits to the target chosen before the file read",async()=>{
  const original={baseUrl:'http://fixture.invalid:8188',workflow:{original_image:true},visual_identity_workflow:{original_identity:true}};
  const store={settings:{values:{'media.comfyui':original}},saving:false,settingsError:''};
  const {state}=await componentHarness('settings/MediaSettings.vue',store,'importFile,importTarget,workflow,visualWorkflow,notice,importing');
  let release;const text=new Promise(resolve=>release=resolve);
  const pending=state.importFile({target:{files:[{name:'image.json',size:30,text:()=>text}],value:'selected'}});
  assert.equal(state.importing.value,true);state.importTarget.value='identity';release('{"1":{"class_type":"Image"}}');await pending;
  assert.equal(JSON.parse(state.workflow.value)['1'].class_type,'Image');assert.deepEqual(JSON.parse(state.visualWorkflow.value),original.visual_identity_workflow);
  assert.match(state.notice.value,/图片工作流草稿/);assert.equal(state.importing.value,false);
});

test('shared choices preserve empty domain values and only emit configured options',async()=>{
  const {state,events}=await componentHarness('product/ChoiceSelect.vue',{},'choose,choiceValue',{modelValue:'',options:[{value:'',label:'不适用'},{value:'shoes',label:'鞋履'}]});
  state.choose(state.choiceValue(''));state.choose(state.choiceValue('shoes'));state.choose('unknown');
  assert.deepEqual(events,[['update:modelValue',''],['update:modelValue','shoes']]);
});

test("cleared visual workflow cannot reappear from a stale successful-save draft",async()=>{
  const original={baseUrl:'http://fixture.invalid:8188',workflow:{'1':{class_type:'Image'}},visual_identity_workflow:{'2':{class_type:'Identity'}},media_workflows:{back:{keep:true}}};
  const saves=[];const store={settings:{values:{'media.comfyui':original}},saving:false,settingsError:'',async saveSettings(values){saves.push(configObject(values));this.settings.values={...this.settings.values,...values};}};
  const {state}=await componentHarness('settings/MediaSettings.vue',store,'save,visualDefault,visualWorkflow,baseUrl');
  state.visualDefault.value=true;await state.save();assert.equal(state.visualWorkflow.value,'');assert.equal(state.visualDefault.value,false);
  state.baseUrl.value='http://another-fixture.invalid:8188';await state.save();assert.equal(saves[1]['media.comfyui'].visual_identity_workflow,undefined);assert.deepEqual(saves[1]['media.comfyui'].media_workflows,original.media_workflows);
});

test("Kev continuation uses the applied filter snapshot instead of unsubmitted edits",async()=>{
  const calls=[];const store={kevDecisionRows:[],kevDecisionCursor:'page-a',kevDecisionError:'',kevDecisionLoading:false,diagnosticDisplayTimezone:'Asia/Shanghai',async loadKevDecisions(filter,reset){calls.push({filter,reset});this.kevDecisionFilter=JSON.stringify(filter);}};
  const {state}=await componentHarness('KevDiagnostics.vue',store,'load,filter,applied');
  state.filter.actor_self='A';await state.load();const accepted={...state.applied.value};state.filter.actor_self='B';state.filter.agent='new-draft';await state.load(false);
  assert.deepEqual(calls[1],{filter:accepted,reset:false});assert.deepEqual(state.applied.value,accepted);assert.equal(state.filter.actor_self,'B');
});

test("Kev invalid date ranges focus the inline error and never request records",async()=>{
  let calls=0;const store={kevDecisionRows:[],kevDecisionCursor:'',kevDecisionError:'',kevDecisionLoading:false,diagnosticDisplayTimezone:'Asia/Shanghai',loadKevDecisions:async()=>calls++};
  const {state,focused}=await componentHarness('KevDiagnostics.vue',store,'load,fromLocal,toLocal,dateErrors');
  state.fromLocal.value='2026-10-13T12:00';state.toLocal.value='2026-10-13T10:00';await state.load();assert.equal(calls,0);assert.match(state.dateErrors.value['kev-to'],/不能早于/);assert.deepEqual(focused,['kev-to']);
});

test('authentication forms show inline policy errors before emitting requests and preserve exact credential text',async()=>{
 const {state,focused,events}=await componentHarness('auth/AuthPanel.vue',{},'submitSignIn,submitSetup,authPassword,setupToken,newOwnerPassword',{loading:false});
 state.authPassword.value='short';await state.submitSignIn();assert.equal(events.length,0);assert.deepEqual(focused,['auth-password']);
 state.authPassword.value='  abc ';await state.submitSignIn();assert.deepEqual(events,[['signIn','  abc ']]);
 state.setupToken.value='';state.newOwnerPassword.value='fixture-password';await state.submitSetup();assert.equal(events.length,1);assert.equal(focused.at(-1),'setup-token');
});

test('lifecycle filters remain a draft until applied and explicit reset clears the applied snapshot',async()=>{
  const source=await readFile(new URL('../src/views/DiagnosticsView.vue',import.meta.url),'utf8');
  const script=source.slice(source.indexOf('const lifecycleDraft'),source.indexOf('function openWorkflow'));
  const js=ts.transpileModule(script,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.None}}).outputText;
  const queries=[];
  const controlCenter=reactive({diagnosticsFluctlightFilter:'',diagnosticsCorrelationFilter:'',diagnosticsIntentFilter:'',diagnosticsWorkflowFilter:'',diagnosticsRunFilter:'',diagnosticsSurfaceFilter:'',diagnosticsStatusFilter:'',loadDiagnostics(){queries.push(this.diagnosticsStatusFilter);}});
  const {lifecycleDraft,applyDiagnosticsFilters,clearLifecycleFilters}=new Function('reactive','watch','controlCenter',`${js};return {lifecycleDraft,applyDiagnosticsFilters,clearLifecycleFilters};`)(reactive,watch,controlCenter);
  lifecycleDraft.status='failed';controlCenter.loadDiagnostics();assert.deepEqual(queries,['']);
  applyDiagnosticsFilters();await nextTick();assert.equal(controlCenter.diagnosticsStatusFilter,'failed');
  lifecycleDraft.status='retry';controlCenter.loadDiagnostics();assert.equal(queries.at(-1),'failed');
  clearLifecycleFilters();await nextTick();assert.equal(queries.at(-1),'');assert.equal(lifecycleDraft.status,'');
  controlCenter.diagnosticsCorrelationFilter='external-link';await nextTick();assert.equal(lifecycleDraft.correlationId,'external-link');
});

test('changing model roles clears old validation and fences late save feedback',async()=>{
  let release,started;const requestStarted=new Promise(resolve=>started=resolve),pending=new Promise(resolve=>release=resolve);
  const store={providerEndpoints:[{id:'fixture'}],providerBindings:[{role:'generic_llm',endpoint_id:'fixture',model_id:'model-a',token_budget:10,timeout_seconds:10},{role:'embedding',endpoint_id:'fixture',model_id:'model-b',token_budget:10,timeout_seconds:10}],settingsError:'',loadProviderModels:async()=>{},configureModelRole:async()=>{started();await pending;store.settingsError='旧角色保存失败';}};
  const {state}=await componentHarness('../views/SettingsView.vue',store,'selectRole,saveRole,roleModelId,errors,localError,localNotice',{section:'model-role'});
  await state.selectRole('generic_llm');state.roleModelId.value='';await state.saveRole();assert.match(state.errors.value['role-model'],/填写/);
  await state.selectRole('embedding');assert.deepEqual(state.errors.value,{});const saving=state.saveRole();await requestStarted;await state.selectRole('generic_llm');release();await saving;
  assert.equal(state.localError.value,'');assert.equal(state.localNotice.value,'');
});

test('clear failure stays inside its open dialog and cancelling clears the action error',async()=>{
  const store={diagnosticError:'',clearDiagnostics:async()=>{store.diagnosticError='无法清空诊断信息。';}};
  const {state}=await componentHarness('../views/DiagnosticsView.vue',store,'clearOpen,clearError,confirmClear',{section:'events'});
  state.clearOpen.value=true;await nextTick();await state.confirmClear();assert.equal(state.clearOpen.value,true);assert.match(state.clearError.value,/无法清空/);assert.equal(store.diagnosticError,'');
  state.clearOpen.value=false;await nextTick();assert.equal(state.clearError.value,'');
});

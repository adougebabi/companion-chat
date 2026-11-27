import assert from 'node:assert/strict';
import test from 'node:test';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {configurationFields,conversationReplyDisplayValue} from '../src/v3/proto/components/configuration-fields.js';
import {SavedReplySummary} from '../src/v3/proto/components/configuration-summary.js';
import {React,installDOM,mountReact} from './helpers/react-dom.mjs';
test('unset reply mode defaults to direct and React Select preserves explicit saved modes',async()=>{
 for(const value of [undefined,{}, {temperature:.2}])assert.equal(conversationReplyDisplayValue(value).mode,'direct');
 const saved=conversationReplyDisplayValue({mode:'legacy',temperature:.2});assert.equal(saved.mode,'legacy');assert.equal(saved.temperature,.2);
 assert.equal(conversationReplyDisplayValue({mode:'fact_only'}).mode,'fact_only');
 const environment=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{const {ChoiceSelect}=await vite.ssrLoadModule('/src/react-app/controls.tsx');const field=configurationFields['conversation.reply'].find(field=>field.key==='mode');const options=field.options.map(option=>typeof option==='string'?{value:option,label:option}:option);const children=options.map(option=>React.createElement('option',{key:option.value,value:option.value},option.label));
  const view=await mountReact(vite,ChoiceSelect,{props:{value:'direct',children}});
  try{assert.match(view.host.querySelector('[role="combobox"]').textContent,new RegExp(options.find(option=>option.value==='direct').label));await view.update({value:saved.mode,children});assert.match(view.host.querySelector('[role="combobox"]').textContent,new RegExp(options.find(option=>option.value==='legacy').label));await view.update({value:'fact_only',children});assert.match(view.host.querySelector('[role="combobox"]').textContent,new RegExp(options.find(option=>option.value==='fact_only').label));}finally{await view.dispose();}
 }finally{await vite.close();environment.restore();}
});

test('saved reply summary identifies the actual mode',()=>{
 const catalog=mode=>({effective_values:{'conversation.reply':{mode}},bindings:[{role:'conversation_reply',endpoint_id:'writer',model_id:'writer-model'}]});
 assert.match(SavedReplySummary(catalog(undefined)),/主模型直接回复/);
 assert.match(SavedReplySummary(catalog('direct')),/主模型直接回复/);
 assert.match(SavedReplySummary(catalog('direct')),/主对话任务使用的模型/);
 assert.match(SavedReplySummary(catalog('fact_only')),/独立回复模型/);
 assert.match(SavedReplySummary(catalog('legacy')),/兼容回复路径/);
});

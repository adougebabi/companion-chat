import assert from 'node:assert/strict';
import test from 'node:test';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {configurationFields,conversationReplyDisplayValue} from '../src/v3/proto/components/configuration-fields.js';
import {React,installDOM,mountReact} from './helpers/react-dom.mjs';
test('unset reply mode remains fact_only and React Select preserves an explicit saved legacy value',async()=>{
 for(const value of [undefined,{}, {temperature:.2}])assert.equal(conversationReplyDisplayValue(value).mode,'fact_only');
 const saved=conversationReplyDisplayValue({mode:'legacy',temperature:.2});assert.equal(saved.mode,'legacy');assert.equal(saved.temperature,.2);
 const environment=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{const {ChoiceSelect}=await vite.ssrLoadModule('/src/react-app/controls.tsx');const field=configurationFields['conversation.reply'].find(field=>field.key==='mode');const options=field.options.map(option=>typeof option==='string'?{value:option,label:option}:option);const children=options.map(option=>React.createElement('option',{key:option.value,value:option.value},option.label));
  const view=await mountReact(vite,ChoiceSelect,{props:{value:'fact_only',children}});
  try{assert.match(view.host.querySelector('[role="combobox"]').textContent,new RegExp(options.find(option=>option.value==='fact_only').label));await view.update({value:saved.mode,children});assert.match(view.host.querySelector('[role="combobox"]').textContent,new RegExp(options.find(option=>option.value==='legacy').label));}finally{await view.dispose();}
 }finally{await vite.close();environment.restore();}
});

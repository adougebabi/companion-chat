import assert from 'node:assert/strict';
import {act} from './react-dom.mjs';

export async function reviewCommand(ui,root=document){
 const required=[...root.querySelectorAll('button')].find(node=>node.textContent.trim()==='同步资料，保留输入');
 if(required){await ui.click(required);return;}
 const trigger=root.querySelector('[aria-label="更多表单操作"]');
 assert.ok(trigger,'the current command exposes its secondary synchronization action');
 await act(async()=>trigger.dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowDown',bubbles:true})));
 await ui.flush();
 const item=[...document.querySelectorAll('[role=menuitem]')].find(node=>node.textContent.trim()==='同步资料，保留输入');
 assert.ok(item,'synchronization is available in the actual form menu');
 await ui.click(item);
}

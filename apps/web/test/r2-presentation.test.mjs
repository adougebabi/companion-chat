import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {default:DomainPrimitives}=await vite.ssrLoadModule('/src/react-app/shared/DomainPrimitives.tsx');
test.after(async()=>{await vite.close();env.restore();});
function Harness({html,prepare}){const [root,setRoot]=React.useState(null);return React.createElement(React.Fragment,null,React.createElement('section',{ref:React.useCallback(node=>{if(node){node.innerHTML=html;prepare(node);setRoot(node);}},[])}),React.createElement(DomainPrimitives,{root}));}
const visible=(root,selector)=>[...root.querySelectorAll(selector)].filter(node=>!node.closest('[data-domain-carrier]'));
test('Card Alert and Item style the original hosts, retaining form ancestry, exact FormData, handlers and content redraws',async()=>{
 let form,field,message,row,submitted;const html='<form data-domain-ui="card" class="card stack"><label>名称<input data-domain-ui="field" name="name" value="原名称"></label><p data-domain-ui="alert" class="banner err" role="alert" data-error>原失败</p><a data-domain-ui="item" class="lrow" href="#/original" data-row>来源记录</a><button data-domain-ui="button" class="btn primary" type="submit">保存</button></form>';
 const v=await mountReact(vite,Harness,{props:{html,prepare:root=>{form=root.querySelector('form');field=form.elements.name;message=form.querySelector('[data-error]');row=form.querySelector('[data-row]');form.onsubmit=event=>{event.preventDefault();submitted=[...new window.FormData(form)];};}}});
 try{assert.equal(form.dataset.slot,'card');assert.equal(form.classList.contains('card'),false);assert.equal(form.querySelector('[name=name]'),field);assert.equal(field.form,form);assert.equal(message.dataset.slot,'alert');assert.equal(message.parentElement,form);assert.equal(row.dataset.slot,'item');assert.equal(row.getAttribute('href'),'#/original');await v.input(visible(v.host,'input')[0],'新名称');await v.click(visible(v.host,'button').find(node=>node.textContent==='保存'));assert.deepEqual(submitted,[['name','新名称']]);
  await act(async()=>{message.className='banner info';message.textContent='已核对，原输入保留';});await v.flush();assert.equal(form.querySelector('[data-error]'),message);assert.equal(message.classList.contains('banner'),false);assert.match(message.textContent,/原输入保留/);
  await act(async()=>{form.innerHTML='<input data-domain-ui="field" name="name" value="重读后"><button data-domain-ui="button" type="submit" class="btn">保存新页</button>';});await v.flush();assert.equal(form.elements.name.form,form);await v.click(visible(v.host,'button').find(node=>node.textContent==='保存新页'));assert.deepEqual(submitted,[['name','重读后']]);assert.equal(v.host.querySelectorAll('[data-slot=item]').length,0);
 }finally{await v.dispose();}
});
test('ToggleGroup preserves the empty domain filter and Pagination forwards only actual cursor actions',async()=>{
 let root,filter='',next=0;const html='<div data-domain-ui="toggle-group" aria-label="生活内容"><button class="on" data-life-filter="" aria-pressed="true">全部</button><button data-life-filter="diary" aria-pressed="false">日记</button></div><div data-domain-ui="pagination"><button data-page-previous disabled>上一页</button><button data-page-next>下一页</button></div>';
 const v=await mountReact(vite,Harness,{props:{html,prepare:node=>{root=node;node.querySelectorAll('[data-life-filter]').forEach(button=>button.onclick=()=>{filter=button.dataset.lifeFilter;node.querySelectorAll('[data-life-filter]').forEach(peer=>{peer.classList.toggle('on',peer===button);peer.setAttribute('aria-pressed',String(peer===button));});});node.querySelector('[data-page-next]').onclick=()=>next++;}}});
 try{assert.equal(filter,'');await v.click(visible(v.host,'button').find(node=>node.textContent==='日记'));assert.equal(filter,'diary');await v.click(visible(v.host,'button').find(node=>node.textContent==='全部'));assert.equal(filter,'');const nav=v.host.querySelector('[data-slot=pagination]');assert.ok(nav);const buttons=[...nav.querySelectorAll('button')];assert.equal(buttons[0].disabled,true);await v.click(buttons[1]);assert.equal(next,1);assert.doesNotMatch(nav.textContent,/第\s*\d+\s*页/);assert.equal(root.querySelector('[data-life-filter=""]').dataset.lifeFilter,'');}finally{await v.dispose();}
});
test('time InputGroup preserves original value and controller focus targets the real text input',async()=>{
 let original;const v=await mountReact(vite,Harness,{props:{html:'<label>开始时间<input data-domain-ui="field" name="start" type="time" value="09:15" required></label>',prepare:root=>original=root.querySelector('[name=start]')}});
 try{const input=visible(v.host,'input')[0];assert.equal(input.dataset.slot,'input-group-control');assert.ok(input.closest('[data-slot=input-group]'));await act(async()=>original.focus());assert.equal(document.activeElement,input);await v.input(input,'12:45');assert.equal(original.value,'12:45');assert.equal(input.value,'12:45');}finally{await v.dispose();}
});
test('file picker exposes one labelled selection action while the native file transport remains hidden',async()=>{
 const {default:FilePicker}=await vite.ssrLoadModule('/src/react-app/shared/FilePicker.tsx');const v=await mountReact(vite,FilePicker,{props:{id:'select-media','aria-label':'选择上传的媒体文件'}});
 try{const button=v.host.querySelector('button'),input=v.host.querySelector('input[type=file]');assert.equal(button.id,'select-media');assert.equal(button.getAttribute('aria-label'),'选择上传的媒体文件');assert.equal(input.hidden,true);assert.equal(input.tabIndex,-1);assert.equal(input.getAttribute('aria-hidden'),'true');assert.notEqual(input.id,button.id);}finally{await v.dispose();}
});
test('media upload also hides its native file transport and provides one contextual selection button',async()=>{
 const {default:MediaUpload}=await vite.ssrLoadModule('/src/react-app/shared/MediaUpload.tsx');const v=await mountReact(vite,MediaUpload,{workspace:{state:{session:{actorId:'r2-upload-owner'}},services:{mediaClient:{}},navigatePath(){}}});
 try{const input=v.host.querySelector('input[type=file]'),buttons=v.host.querySelectorAll('button[aria-label="选择上传的媒体文件"]');assert.equal(input.hidden,true);assert.equal(input.getAttribute('aria-hidden'),'true');assert.equal(buttons.length,1);}finally{await v.dispose();}
});

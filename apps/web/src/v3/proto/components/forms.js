import { esc, icon } from './controls.js';

// Small, framework-neutral view components match the native v3 source. Slots
// contain component-produced markup; user text always goes through esc().
export function attributes(values = {}) {
  return Object.entries(values).filter(([, value]) => value !== false && value !== undefined && value !== null)
    .map(([key, value]) => {
      if (!/^[a-zA-Z][\w:-]*$/.test(key) || /^on/i.test(key)) throw new Error('Unsafe component attribute');
      return value === true ? key : `${key}="${esc(value)}"`;
    }).join(' ');
}
export function Button({ label = '', content, tone = '', size = '', type = 'button', attrs = {} } = {}) {
  return `<button class="btn${tone ? ` ${esc(tone)}` : ''}${size ? ` ${esc(size)}` : ''}" type="${esc(type)}" ${attributes(attrs)}>${content ?? esc(label)}</button>`;
}
let fieldSequence = 0;
export function Field({ label, name, value = '', type = 'text', options, rows = 3, hint = '', attrs = {}, labelAttrs = {} }) {
  const id = attrs.id || `field-${++fieldSequence}`;
  const describedBy = [attrs['aria-describedby'], hint ? `${id}-hint` : ''].filter(Boolean).join(' ');
  const common = attributes({ name, ...(type === 'number' ? { step: 'any' } : {}), ...attrs, id,
    ...(attrs.required&&!attrs.readonly&&!attrs.disabled ? { 'aria-required': 'true' } : {}), ...(describedBy ? { 'aria-describedby': describedBy } : {}) });
  const control = type === 'textarea' ? `<textarea class="input" rows="${rows}" ${common}>${esc(value)}</textarea>`
    : type === 'select' ? `<select class="select" ${common}>${options.map(option => { const item = typeof option === 'string' ? { value: option, label: option } : option; return `<option value="${esc(item.value)}"${String(item.value) === String(value) ? ' selected' : ''}>${esc(item.label)}</option>`; }).join('')}</select>`
      : `<input class="input" type="${esc(type)}" value="${esc(value)}" ${common}>`;
  return `<label class="field" ${attributes(labelAttrs)}><span class="field-label">${esc(label)}${attrs.required&&!attrs.readonly&&!attrs.disabled ? '<span class="field-required" aria-hidden="true" title="必填">*</span>' : ''}</span>${control}${hint ? `<span class="small muted" id="${esc(id)}-hint">${esc(hint)}</span>` : ''}</label>`;
}
export function Tabs({ items, value, attribute = 'tab', className = 'tabs', attrs = {} }) {
  return `<div class="${esc(className)}" ${attributes(attrs)}>${items.map(item => {
    const cls = `tab ${item.value === value ? 'on' : ''}`;
    const body = item.content ?? esc(item.label);
    return item.href ? `<a class="${cls}" href="${esc(item.href)}" ${attributes({ ...(item.value === value ? { 'aria-current': 'page' } : {}), ...(item.attrs || {}) })}>${body}</a>` : `<button type="button" class="${cls}" ${attributes({ [`data-${attribute}`]: item.value, 'aria-pressed': String(item.value === value), ...(item.attrs || {}) })}>${body}</button>`;
  }).join('')}</div>`;
}
export function SearchForm({ value = '', placeholder = '搜索', submitLabel = '搜索', attrs = {}, inputAttrs = {}, inputClass = 'input grow', buttonClass = 'btn' } = {}) {
  return `<form class="row" role="search" ${attributes(attrs)}><input class="${esc(inputClass)}" name="q" value="${esc(value)}" placeholder="${esc(placeholder)}" ${attributes({ 'aria-label': placeholder, ...inputAttrs })}><button class="${esc(buttonClass)}">${esc(submitLabel)}</button></form>`;
}
export function PromptChips(items, attribute = 'chip', className = 'chips') {
  return `<div class="${esc(className)}">${items.map(text => `<button class="chip" data-${esc(attribute)}="${esc(text)}">${esc(text)}</button>`).join('')}</div>`;
}
export function TextComposer({ variant = 'chat', placeholder = '', attrs = {}, name = 't' } = {}) {
  if (variant === 'assistant') return `<form class="row" ${attributes(attrs)}><input class="input grow" name="${esc(name)}" aria-label="创作请求" placeholder="${esc(placeholder)}" data-test="assist-input"><button class="btn accent sm" aria-label="提交创作请求">${icon('send')}</button></form>`;
  return `<form class="composer" data-test="composer" ${attributes(attrs)}><textarea class="input" rows="1" name="${esc(name)}" placeholder="${esc(placeholder)}" aria-label="消息" enterkeyhint="send" data-test="chat-input"></textarea><button class="btn primary" data-test="chat-send">${icon('send')} 发送</button></form>`;
}
export function DiffView(rows, { attrs = {}, emptyText = '无差异' } = {}) {
  return `<div class="diff" ${attributes(attrs)}>${DiffRows(rows, emptyText)}</div>`;
}
export function DiffRows(rows, emptyText = '无差异') {
  return rows.filter(row=>row.before!==row.after).map(row => `${row.field ? `<div class="ctx">${esc(row.field)}</div>` : ''}<div class="del">- ${esc(row.before)}</div><div class="add">+ ${esc(row.after)}</div>`).join('') || `<div class="ctx">${esc(emptyText)}</div>`;
}
export function PreferenceFields(model) {
  return Object.entries(model).filter(([key]) => key !== 'savedAt').map(([key, value]) => typeof value === 'boolean'
    ? `<label class="toggle"><input type="checkbox" name="${esc(key)}" ${value ? 'checked' : ''}> ${esc(key)}</label>`
    : `<label class="field">${esc(key)}<input class="input" name="${esc(key)}" value="${esc(value)}"></label>`).join('');
}
export function Toggle({label,name,checked=false,attrs={}}) {
  return `<label class="toggle"><input type="checkbox" ${attributes({name,checked,...attrs})}> ${esc(label)}</label>`;
}
export function CheckboxGroup({label,items,value=[],attribute,className=''}) {
  return `<div class="${esc(className)}" role="group" aria-label="${esc(label)}"><b>${esc(label)}</b>${items.map(([key,title])=>Toggle({label:title,checked:value.includes(key),attrs:{[`data-${attribute}`]:key}})).join('')}</div>`;
}
export function KeyValueList(entries, { className = 'kv', attrs = {} } = {}) {
  return `<div class="${esc(className)}" ${attributes(attrs)}>${entries.map(([label, content]) => `<dt>${esc(label)}</dt><dd>${content}</dd>`).join('')}</div>`;
}

export function constraintMessage(control) {
 const validity=control.validity||{};
 if(validity.valueMissing || (control.required && typeof control.value==='string' && !control.value.trim()))return '请填写此项。';
 if(validity.typeMismatch)return control.type==='email'?'请输入有效的邮箱地址。':'请输入有效的网址。';
 if(validity.rangeUnderflow)return `请输入不小于 ${control.min} 的数值。`;
 if(validity.rangeOverflow)return `请输入不大于 ${control.max} 的数值。`;
 if(validity.tooShort)return `请至少填写 ${control.minLength} 个字符。`;
 if(validity.tooLong)return `请最多填写 ${control.maxLength} 个字符。`;
 if(validity.badInput || validity.stepMismatch)return '请输入符合要求的数值。';
 if(validity.patternMismatch)return '请按字段要求填写。';
 if(validity.customError && /[\u3400-\u9fff]/u.test(control.validationMessage))return control.validationMessage;
 return '请检查此项的填写格式。';
}
// Capture native validation, including explicit reportValidity(), without changing
// native prototypes or relaxing constraints. Form owners keep their submit logic.
export function installFieldValidation(root=document) {
 let firstInvalid=null, scheduled=false;
 const errors=new WeakMap(), whitespaceErrors=new WeakSet();
 const syncRequired=control=>{
  if(typeof control?.setCustomValidity!=='function')return;
  // Only clear the constraint installed here; page-owned custom errors survive.
  if(whitespaceErrors.has(control)){
   if(control.validationMessage==='请填写此项。')control.setCustomValidity('');
   whitespaceErrors.delete(control);
  }
  if(control.willValidate&&control.required&&typeof control.value==='string'&&control.value&&!control.value.trim()&&!control.validity.customError){
   control.setCustomValidity('请填写此项。');whitespaceErrors.add(control);
  }
 };
 const syncAll=()=>root.querySelectorAll?.('input,textarea,select').forEach(syncRequired);
 // Footer actions often call reportValidity without dispatching submit. Capture
 // their activation before their handler and before native constraint checks.
 const activate=event=>{if(event.type==='click'||event.key==='Enter')syncAll();};
 const show=control=>{
  let error=errors.get(control);
  if(!error){error=document.createElement('span');error.className='field-error';error.id=`constraint-${++fieldSequence}`;error.setAttribute('role','alert');control.after(error);errors.set(control,error);}
  error.textContent=constraintMessage(control);error.hidden=false;
  control.setAttribute('aria-invalid','true');
  const ids=new Set((control.getAttribute('aria-describedby')||'').split(/\s+/).filter(Boolean));ids.add(error.id);control.setAttribute('aria-describedby',[...ids].join(' '));
  firstInvalid??=control;
  if(!scheduled){scheduled=true;queueMicrotask(()=>{scheduled=false;const field=firstInvalid;firstInvalid=null;if(field?.isConnected){field.focus();field.scrollIntoView?.({block:'center',behavior:'auto'});}});}
 };
 const invalid=event=>{event.preventDefault();show(event.target);};
 const input=event=>{const control=event.target;syncRequired(control);const error=errors.get(control);if(error){error.hidden=true;control.removeAttribute('aria-invalid');const ids=(control.getAttribute('aria-describedby')||'').split(/\s+/).filter(id=>id!==error.id);if(ids.length)control.setAttribute('aria-describedby',ids.join(' '));else control.removeAttribute('aria-describedby');}};
 const submit=event=>{const controls=[...(event.target.elements||[])].filter(control=>control.willValidate&&control.required&&typeof control.value==='string'&&!control.value.trim());if(controls.length){event.preventDefault();event.stopImmediatePropagation();controls.forEach(show);}};
 root.addEventListener('click',activate,true);root.addEventListener('keydown',activate,true);root.addEventListener('invalid',invalid,true);root.addEventListener('input',input,true);root.addEventListener('change',input,true);root.addEventListener('submit',submit,true);
 return()=>{root.removeEventListener('click',activate,true);root.removeEventListener('keydown',activate,true);root.removeEventListener('invalid',invalid,true);root.removeEventListener('input',input,true);root.removeEventListener('change',input,true);root.removeEventListener('submit',submit,true);};
}

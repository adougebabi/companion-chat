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
export function Field({ label, name, value = '', type = 'text', options, rows = 3, hint = '', attrs = {}, labelAttrs = {} }) {
  const common = attributes({ name, ...(type === 'number' ? { step: 'any' } : {}), ...attrs });
  const control = type === 'textarea' ? `<textarea class="input" rows="${rows}" ${common}>${esc(value)}</textarea>`
    : type === 'select' ? `<select class="select" ${common}>${options.map(option => { const item = typeof option === 'string' ? { value: option, label: option } : option; return `<option value="${esc(item.value)}"${String(item.value) === String(value) ? ' selected' : ''}>${esc(item.label)}</option>`; }).join('')}</select>`
      : `<input class="input" type="${esc(type)}" value="${esc(value)}" ${common}>`;
  return `<label class="field" ${attributes(labelAttrs)}>${esc(label)}${control}${hint ? `<span class="small muted">${esc(hint)}</span>` : ''}</label>`;
}
export function Tabs({ items, value, attribute = 'tab', className = 'tabs', attrs = {} }) {
  return `<div class="${esc(className)}" ${attributes(attrs)}>${items.map(item => {
    const cls = `tab ${item.value === value ? 'on' : ''}`;
    const body = item.content ?? esc(item.label);
    return item.href ? `<a class="${cls}" href="${esc(item.href)}" ${attributes(item.attrs || {})}>${body}</a>` : `<button type="button" class="${cls}" ${attributes({ [`data-${attribute}`]: item.value, ...(item.attrs || {}) })}>${body}</button>`;
  }).join('')}</div>`;
}
export function SearchForm({ value = '', placeholder = '搜索', submitLabel = '搜索', attrs = {}, inputAttrs = {}, inputClass = 'input grow', buttonClass = 'btn' } = {}) {
  return `<form class="row" ${attributes(attrs)}><input class="${esc(inputClass)}" name="q" value="${esc(value)}" placeholder="${esc(placeholder)}" ${attributes(inputAttrs)}><button class="${esc(buttonClass)}">${esc(submitLabel)}</button></form>`;
}
export function PromptChips(items, attribute = 'chip', className = 'chips') {
  return `<div class="${esc(className)}">${items.map(text => `<button class="chip" data-${esc(attribute)}="${esc(text)}">${esc(text)}</button>`).join('')}</div>`;
}
export function TextComposer({ variant = 'chat', placeholder = '', attrs = {}, name = 't' } = {}) {
  if (variant === 'assistant') return `<form class="row" ${attributes(attrs)}><input class="input grow" name="${esc(name)}" placeholder="${esc(placeholder)}" data-test="assist-input"><button class="btn accent sm">${icon('send')}</button></form>`;
  return `<form class="composer" data-test="composer" ${attributes(attrs)}><input class="input" name="${esc(name)}" placeholder="${esc(placeholder)}" autocomplete="off" data-test="chat-input"><button class="btn primary" data-test="chat-send">${icon('send')} 发送</button></form>`;
}
export function DiffView(rows, { attrs = {}, emptyText = '无差异' } = {}) {
  return `<div class="diff" ${attributes(attrs)}>${DiffRows(rows, emptyText)}</div>`;
}
export function DiffRows(rows, emptyText = '无差异') {
  return rows.map(row => `${row.field ? `<div class="ctx">${esc(row.field)}</div>` : ''}<div class="del">- ${esc(row.before)}</div><div class="add">+ ${esc(row.after)}</div>`).join('') || `<div class="ctx">${esc(emptyText)}</div>`;
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

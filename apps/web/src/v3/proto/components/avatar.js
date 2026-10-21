import {esc} from './controls.js';

// Keep a neutral, name-based fallback behind the real image, including load failures.
export function Avatar({src='',name='对方',size=''}={}) {
  const initial=Array.from(String(name).trim())[0] || '人';
  return `<span class="av ${['lg','sm'].includes(size)?size+' ':''}avatar" role="img" aria-label="${esc(name)}的头像"><span class="avatar-fallback" aria-hidden="true">${esc(initial)}</span>${typeof src==='string'&&src.trim()?`<img src="${esc(src)}" alt="" data-avatar-image>`:''}</span>`;
}
export function bindAvatars(root) {
  root.querySelectorAll('[data-avatar-image]').forEach(image=>{
    image.addEventListener('error',()=>image.remove(),{once:true});
    if(image.complete&&!image.naturalWidth)image.remove();
  });
}

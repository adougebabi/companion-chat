// Read-only production-DOM measurements. This helper is never imported by src/.
// Use the actual built UI; JSDOM cannot certify geometry.
export function readR12Layout(){
 const text=node=>(node?.textContent||'').replace(/\s+/g,' ').trim();
 const shown=node=>{const rect=node.getBoundingClientRect();return rect.width>0&&rect.height>0&&!node.closest('[hidden],[data-domain-carrier]');};
 const rect=node=>{const r=node.getBoundingClientRect();return {x:+r.x.toFixed(2),y:+r.y.toFixed(2),w:+r.width.toFixed(2),h:+r.height.toFixed(2),right:+r.right.toFixed(2)};};
 const all=selector=>[...document.querySelectorAll(selector)].filter(shown);
 const cards=selector=>all(selector).map(node=>({name:text(node.querySelector('[data-slot=card-title]')), ...rect(node)}));
 const actionNames=['继续创建与查看进度','重新读取准备状态','查看与规划日程','重新读取已有世界','创建世界设定','保留草稿并返回角色库','测试连接'];
 const actions=all('[data-slot=button]').filter(node=>actionNames.some(name=>text(node).includes(name))).map(node=>({text:text(node),...rect(node),parentW:+node.parentElement.getBoundingClientRect().width.toFixed(2)}));
 const trigger=all('[aria-haspopup=menu][aria-expanded=true]')[0];
 const menu=all('[data-slot=dropdown-menu-content]')[0];
 const sidebar=document.querySelector('[data-slot=sidebar-container]')||document.querySelector('[data-slot=sidebar][data-mobile=true]');
 const nav=all('[data-sidebar=menu-button],[data-sidebar=menu-sub-button]');
 const fields=all('input').filter(node=>node.type==='number'||node.type==='date'||node.type==='time'||node.getAttribute('inputmode')==='numeric').map(node=>({type:node.type,name:node.getAttribute('aria-label')||node.name,appearance:getComputedStyle(node).appearance,pattern:node.pattern}));
 const gridColumns=selector=>{const node=document.querySelector(selector);return node?getComputedStyle(node).gridTemplateColumns.split(' ').filter(Boolean).length:0;};
 return {
  route:location.hash,viewport:{w:innerWidth,h:innerHeight,overflow:document.documentElement.scrollWidth-document.documentElement.clientWidth},
  ready:!!document.querySelector('main')&&!document.querySelector('[role=status] [data-slot=skeleton]'),
  library:cards('[data-testid^="library-card-"]'),worlds:cards('[data-testid^="world-card-"]'),libraryColumns:gridColumns('[data-testid=library-grid]'),worldColumns:gridColumns('[data-testid=world-grid]'),actions,
  sidebar:sidebar?{...rect(sidebar),menuButtons:nav.length,fontSizes:[...new Set(nav.map(node=>getComputedStyle(node).fontSize))],groups:all('[data-slot=sidebar-group-label]').map(text),active:nav.filter(node=>node.dataset.active==='true').map(text),unintendedActive:nav.filter(node=>node.dataset.active==='false'&&!node.matches(':hover')&&!['rgba(0, 0, 0, 0)','transparent'].includes(getComputedStyle(node).backgroundColor)).map(text)}:null,
  menu:menu?{align:menu.getAttribute('data-align'),...rect(menu),trigger:trigger?{...rect(trigger),kind:trigger.classList.contains('character-switch')?'switcher':'more',label:trigger.getAttribute('aria-label')||text(trigger)}:null}:null,
  calendars:all('[data-slot=calendar]').length,dateControls:all('[data-slot=popover-trigger][role=combobox]').map(text),alerts:all('[data-slot=alert]').map(text),fields,invalidTitles:all('[title]').filter(node=>node.getAttribute('title')==='[object Object]').length,nativeProgress:all('progress').length,disclosureRows:all('.disclosure-summary').length,
  confirmations:all('[role=alertdialog]').map(node=>({title:text(node.querySelector('[data-slot=alert-dialog-title]')),footer:!!node.querySelector('[data-slot=alert-dialog-footer]'),actions:node.querySelectorAll('[data-slot=alert-dialog-action]').length,cancels:node.querySelectorAll('[data-slot=alert-dialog-cancel]').length})),
 };
}

export function validateR12Layout(value,{libraryCount,worldCount,menu=false,confirmation=false}={}){
 const issues=[];
 if(value.viewport.overflow>1)issues.push('horizontal overflow '+value.viewport.overflow);
 for(const [name,expected,columns]of [['library',libraryCount,value.libraryColumns],['worlds',worldCount,value.worldColumns]]){
  if(expected!==undefined&&value[name].length!==expected)issues.push(name+' count '+value[name].length+' expected '+expected);
  if(value[name].some(card=>card.w>381))issues.push(name+' card exceeds 380px');
  if(expected>0&&value.viewport.w>=1000&&columns<2)issues.push(name+' is not a multi-column grid');
 }
 for(const action of value.actions)if(action.w>400||action.parentW>450&&action.w/action.parentW>.8)issues.push('stretched action: '+action.text+' ('+action.w+'/'+action.parentW+')');
 if(value.sidebar&&value.viewport.w>=1000&&value.sidebar.w<240)issues.push('sidebar too narrow');
 if(value.sidebar?.fontSizes?.some(size=>parseFloat(size)<13.9))issues.push('sidebar text is below 14px');
 if(value.sidebar?.unintendedActive.length)issues.push('inactive navigation highlighted: '+value.sidebar.unintendedActive.join(','));
 if(value.fields.some(field=>['date','time'].includes(field.type)))issues.push('visible browser date/time picker');
 if(value.fields.some(field=>field.type==='number'&&field.appearance!=='textfield'))issues.push('number spinner is not suppressed');
 if(value.invalidTitles)issues.push('invalid object title');
 if(value.nativeProgress)issues.push('native progress');
 if(menu){const popup=value.menu;if(!popup)issues.push('menu did not open');else{const expected=popup.trigger?.kind==='switcher'?'start':'end';if(popup.align!==expected)issues.push('menu align '+popup.align+' expected '+expected);if(popup.x<0||popup.right>value.viewport.w+1)issues.push('menu outside viewport');if(expected==='end'&&popup.trigger&&Math.abs(popup.right-popup.trigger.right)>3)issues.push('more menu is not right aligned');}}
 if(confirmation&&!value.confirmations.some(dialog=>dialog.footer&&dialog.actions===1&&dialog.cancels>=1))issues.push('incomplete semantic confirmation');
 return issues;
}

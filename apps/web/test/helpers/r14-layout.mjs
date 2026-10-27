// Read-only production-browser geometry; JSDOM cannot certify these dimensions.
export function readR14Layout() {
  const rect = node => {
    if (!node) return null;
    const r = node.getBoundingClientRect();
    return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};
  };
  const visible = node => !!(node && node.getClientRects().length && getComputedStyle(node).visibility !== 'hidden');
  return {
    route: location.hash,
    viewport: {width:innerWidth,height:innerHeight},
    document: {width:document.documentElement.scrollWidth,height:document.documentElement.scrollHeight},
    sidebar: rect(document.querySelector('[data-slot="sidebar-container"]')),
    rail: !!document.querySelector('[data-slot="sidebar-rail"]'),
    blockedNavigation: document.querySelector('[role="dialog"],[role="menu"]')?[]:[...document.querySelectorAll('[data-sidebar="menu-button"]')].filter(node=>{
      const r=node.getBoundingClientRect();
      return visible(node)&&r.width>0&&r.height>0&&r.x>=0&&r.y>=0&&r.right<=innerWidth&&r.bottom<=innerHeight&&document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)?.closest('button')!==node;
    }).map(node=>node.textContent.trim()),
    preview: rect(document.querySelector('.chat-preview-sidebar')),
    composer: rect(document.querySelector('.composer')),
    messages: document.querySelectorAll('[data-slot="message"]').length,
    calendars: [...document.querySelectorAll('[data-slot="calendar"]')].filter(visible).map(node => ({
      frame:rect(node),table:rect(node.querySelector('table')),
      days:[...node.querySelectorAll('button[data-day]')].filter(visible).map(rect),
    })),
    tabs: [...document.querySelectorAll('[data-slot="tabs-list"]')].filter(visible).map(node => ({
      frame:rect(node),width:node.clientWidth,scrollWidth:node.scrollWidth,overflowX:getComputedStyle(node).overflowX,
    })),
    switches:[...document.querySelectorAll('[role="switch"]')].filter(visible).map(node => ({
      frame:rect(node),state:node.getAttribute('data-state'),
      before:getComputedStyle(node,'::before').content,
      thumbs:node.querySelectorAll('[data-slot="switch-thumb"]').length,
      translate:getComputedStyle(node.querySelector('[data-slot="switch-thumb"]')).translate,
    })),
    dialogs:[...document.querySelectorAll('[role="dialog"][data-slot="sheet-content"],[role="dialog"][data-slot="dialog-content"]')].filter(visible).map(node => ({
      frame:rect(node),closes:[...node.querySelectorAll('button')].filter(button=>['关闭','关闭预览','Close'].includes(button.getAttribute('aria-label')||button.textContent.trim())).length,
    })),
  };
}
export function validateR14Layout(data) {
  const errors=[];
  if(data.blockedNavigation.length)errors.push('navigation pointer blocked');
  if(data.document.width>data.viewport.width+1)errors.push('document overflow');
  if(data.preview&&(data.preview.x<0||data.preview.right>data.viewport.width+1))errors.push('preview clipped');
  if(data.composer&&data.composer.bottom>data.viewport.height+1)errors.push('composer below viewport');
  for(const calendar of data.calendars)for(const child of [calendar.table,...calendar.days].filter(Boolean))
    if(child.x<calendar.frame.x-1||child.right>calendar.frame.right+1)errors.push('calendar child clipped');
  for(const tabs of data.tabs)if(tabs.scrollWidth>tabs.width+1)errors.push('tabs overflow');
  for(const control of data.switches)if(!['none','normal'].includes(control.before)||control.thumbs!==1)errors.push('switch double skin');
  for(const dialog of data.dialogs)if(dialog.closes!==1)errors.push('dialog close count');
  return [...new Set(errors)];
}

let handler;
export function configureToast(next){handler=next;return()=>{if(handler===next)handler=undefined;};}
export function toast(text,type=''){handler?.(text,type);}

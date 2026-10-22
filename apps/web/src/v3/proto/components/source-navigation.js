// Local hash routes only. A source target is navigation state, never a write.
export function sourceHref(href,ctx,label='返回来源'){
 const [path,query='']=href.split('?'),params=new URLSearchParams(query);
 params.set('returnTo','#'+ctx.raw);params.set('returnLabel',label);
 return path+'?'+params.toString();
}
export function sourceReturn(ctx,fallback){return typeof ctx.query.returnTo==='string'&&ctx.query.returnTo.startsWith('#/')?ctx.query.returnTo:fallback;}

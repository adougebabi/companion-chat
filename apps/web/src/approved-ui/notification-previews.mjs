// Read only the exact linked message through the authenticated conversation API.
export async function readNotificationPreviews(client,rows,live=()=>true){
 const grouped=new Map(),previews={};
 for(const row of rows.slice(0,30)){
  const link=String(row.link||'').replace(/^#/,'');
  if(!link.startsWith('/')||link.startsWith('//'))continue;
  try{
   const url=new URL(link,'http://notice.local'),match=/^\/c\/([^/]+)\/chat$/.exec(url.pathname);
   const conversation=url.searchParams.get('conversation'),message=url.searchParams.get('message');
   if(!match||decodeURIComponent(match[1])!==row.fluctlight_id||!conversation||!message)continue;
   if(!grouped.has(conversation)){if(grouped.size>=10)continue;grouped.set(conversation,[]);}
   grouped.get(conversation).push({notice:row,message});
  }catch{}
 }
 await Promise.allSettled([...grouped].map(async([conversation,entries])=>{
  if(!live())return;
  const page=await client.messages(conversation);
  if(!live()||page.conversation?.id!==conversation)return;
  for(const {notice,message} of entries){
   const actual=page.messages.find(m=>m.id===message&&m.conversationId===conversation&&m.kind==='assistant'&&m.authorActorId===notice.fluctlight_id);
   if(actual&&typeof actual.text==='string'&&actual.text.trim())previews[notice.id]=actual.text;
  }
 }));
 return live()?previews:{};
}

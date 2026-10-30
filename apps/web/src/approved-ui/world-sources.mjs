export async function reusableWorlds(client,characters,previous=[]){
 const results=await Promise.allSettled(characters.map(async character=>{
  const catalog=await client.worldCatalog(character.id);
  if(catalog?.available===false||!Array.isArray(catalog?.items)||catalog.next_cursor)throw Error('世界目录尚未完整读到');
  return catalog.items.map(world=>({...world,fluctlightId:character.id,characterName:character.name}));
 }));
 const seen=new Set(),items=results.flatMap((result,index)=>result.status==='fulfilled'?result.value:previous.filter(world=>world.fluctlightId===characters[index].id)).filter(world=>{if(seen.has(world.id))return false;seen.add(world.id);return true;});
 return {items,unavailable:results.filter(result=>result.status==='rejected').length,unavailableNames:results.flatMap((result,index)=>result.status==='rejected'?[characters[index].name]:[])};
}

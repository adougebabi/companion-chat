export async function reusableWorlds(client,characters){
  const results=await Promise.allSettled(characters.map(async character=>{
    const catalog=await client.worldCatalog(character.id);
    return catalog.items.map(world=>({...world,fluctlightId:character.id,characterName:character.name}));
  }));
  return {items:results.flatMap(result=>result.status==='fulfilled'?result.value:[]),unavailable:results.filter(result=>result.status==='rejected').length};
}

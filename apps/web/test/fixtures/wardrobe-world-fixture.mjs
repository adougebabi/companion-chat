// Explicit isolated browser replay only. No proxy, model call or production import.
export function wardrobeWorldFixture({fid,owner,date}){
 let revision=3,sequence=0,tags=['在家','外出','上班'];const profile='sunny';
 let items=['upper_body','lower_body','underwear','panties','socks','shoes','accessory','accessory','outer'].map((type,index)=>({id:`clothing-${index}`,fluctlight_id:fid,item_kind:'wearable',category:type,slot:type,clothing_type:type,seasons:['spring','autumn'],description:['青绿棉质上衣','宽松长裤','柔软内衣','棉质内裤','短袜','平底鞋','银色项链','简约手链','薄外套'][index],ownership:'owned',availability:'available',source_kind:'owner_correction',source_ref:'fixture-known',source_item_key:`fixture-${index}`,source_verified:true,revision:1,acquired_at:date,updated_at:date,archived_at:null,media:[],worn:null,use:null}));
 let plans=[];const worn=items.slice(0,6),world={id:'fixture-world',fluctlight_id:fid,name:'温和魔法世界',background:'中世纪魔法与普通生活并存',rules:'尊重居民的日常生活',flavor:'温暖、安静',description:'集市与住家构成的小镇',item_pool:[],revision:2,venue_categories:[{id:'home',name:'家'},{id:'shops',name:'商店'}]};
 let places=[{id:'fixture-bedroom',world_id:world.id,name:'卧室',venue_category_id:'home',alias:'她的房间',floor:'二层',desc:'安静的木质房间',atmosphere:'温暖',furniture:['木床','书桌'],layout:'床靠西墙，窗朝东',connections:[],room_items:[],connection_place_ids:[],parent_place_id:null,revision:1,status:'active'},{id:'fixture-clothing-shop',world_id:world.id,name:'服装店',venue_category_id:'shops',alias:'',floor:'一层',desc:'出售日常服饰的小店',atmosphere:'明亮',furniture:['衣架','柜台'],layout:'中间通道',connections:[],room_items:[],connection_place_ids:[],parent_place_id:null,revision:1,status:'active'}];
 const outfit=p=>({...p,active_profile_id:profile,items:p.itemIds.map(id=>items.find(i=>i.id===id)).filter(Boolean)});
 const snapshot=()=>({available:true,world,places,npcs:[],scenes:[],current_context:{location_known:false},active_effects:[]});
 return function handle({path,method,body,url,json,res}){
  if(!path.startsWith(`/api/fluctlights/${fid}/`))return false;
  const end=path.slice(`/api/fluctlights/${fid}`.length),reply=(value,status=200)=>{json(res,value,status);return true;};
  const conflict=expected=>Number(expected)!==revision;
  if(end==='/inventory/preparation')return reply({id:'preparation-fixture',fluctlight_id:fid,status:'completed',profile_id:profile,result:{inventory_committed:true},task_id:'fixture-wardrobe-task',can_recover:false});
  if(end==='/inventory/tags'){
   if(method==='PUT'){if(conflict(body.expectedRevision))return reply({code:'revision_conflict',message:'衣柜已更新，请重新核对'},409);tags=body.tags;for(const plan of plans){plan.tags=plan.tags.map(t=>body.renames?.[t]||t).filter(t=>tags.includes(t));plan.revision++;}revision++;}
   return reply({tags,revision,fluctlight_id:fid});
  }
  if(end==='/inventory/outfits'){
   if(method==='POST'){if(conflict(body.expectedRevision))return reply({code:'revision_conflict',message:'衣柜已变化'},409);const plan={...body,id:'plan-'+(++sequence),profile_id:profile,fluctlight_id:fid,revision:1};plans.push(plan);revision++;return reply({outfit:outfit(plan),revision});}
   return reply({profile_id:profile,items:plans.map(outfit),next_cursor:''});
  }
  if(end.startsWith('/inventory/outfits/')){const id=end.split('/')[3],plan=plans.find(p=>p.id===id);if(!plan)return reply({code:'not_found',message:'没有此方案'},404);if(end.endsWith('/revisions'))return reply({items:[],next_cursor:''});if(method==='PATCH'||method==='PUT'){if(conflict(body.expectedRevision)||plan.revision!==body.expectedOutfitRevision)return reply({code:'revision_conflict',message:'方案版本已变化'},409);Object.assign(plan,body,{revision:plan.revision+1});revision++;return reply({outfit:outfit(plan),revision});}return reply(outfit(plan));}
  if(end==='/inventory/items'&&method==='POST'){if(conflict(body.expectedRevision))return reply({code:'revision_conflict',message:'衣柜已变化'},409);const v=body.item,item={...items[0],...v,item_kind:v.itemKind,clothing_type:v.clothingType,id:'added-'+(++sequence),worn:null,use:null};items.push(item);revision++;return reply({item,revision});}
  if(end.startsWith('/inventory/items/')){const id=end.split('/')[3],item=items.find(i=>i.id===id);if(!item)return reply({code:'not_found',message:'无此服装'},404);if(end.endsWith('/history'))return reply({items:[],next_cursor:''});if(method==='PUT'){if(conflict(body.expectedRevision))return reply({code:'revision_conflict',message:'衣柜已变化'},409);Object.assign(item,body.item,{clothing_type:body.item.clothingType,revision:item.revision+1});revision++;return reply({item,revision});}if(method==='DELETE'){item.archived_at=date;revision++;return reply({item,revision});}return reply(item);}
  if(end==='/inventory/wearing-history')return reply({items:[],next_cursor:''});
  if(end==='/inventory'){const state=url.searchParams.get('state');return reply({revision,items:items.filter(i=>state==='archived'?i.archived_at:!i.archived_at),worn_items:worn,used_items:[],inventory_complete:false,next_cursor:''});}
  if(end==='/wardrobe')return reply({revision,items,worn_items:worn,used_items:[],inventory_complete:false,next_cursor:'',has_more:false});
  if(end==='/world/catalog')return reply({items:[{...world,active:true}],active_binding:{world_id:world.id,revision:1},available:true});
  if(end==='/world'||end===`/world/catalog/${world.id}`)return reply(snapshot());
  if(end==='/nearby')return reply({available:true,world_id:world.id,location_known:false,place_id:null,actors:[]});
  if(end.startsWith('/world/places/')&&method==='PUT'){const point=places.find(p=>p.id===end.split('/')[3]);if(!point)return reply({code:'not_found',message:'地点不可用'},404);if(point.name!==body.name)return reply({code:'place_name_immutable',message:'正式名称创建后固定'},422);Object.assign(point,body,{desc:body.description,revision:point.revision+1});return reply({id:point.id,revision:point.revision});}
  if(end==='/world/places'&&method==='POST'){const point={...body,id:'place-'+(++sequence),world_id:world.id,venue_category_id:body.venueCategoryId,desc:body.description,revision:1,status:'active'};places.push(point);return reply({id:point.id,revision:1});}
  return false;
 };
}

export type WardrobeItemDraft = { itemKind: "wearable" | "object"; category: string; slot: string; description: string; ownership: string; availability: string };

export const wardrobeSlots = [
 {value:"upper_body",label:"上装"},{value:"lower_body",label:"下装"},{value:"outer",label:"外套"},
 {value:"full_body",label:"连身服装"},{value:"shoes",label:"鞋履"},{value:"socks",label:"袜子"},
 {value:"head",label:"帽子与头饰"},{value:"accessory",label:"配饰"},
];
// Category/slot are extensible backend names. These are safe UI presets, not
// a new domain enum; advanced imports retain existing custom vocabularies.
export const wardrobeCategories = [
 {kind:"wearable",value:"top",label:"上装",slots:["upper_body"]},
 {kind:"wearable",value:"bottom",label:"下装",slots:["lower_body"]},
 {kind:"wearable",value:"coat",label:"外套",slots:["outer"]},
 {kind:"wearable",value:"dress",label:"连衣裙 / 连体装",slots:["full_body"]},
 {kind:"wearable",value:"shoes",label:"鞋",slots:["shoes"]},
 {kind:"wearable",value:"boots",label:"靴子",slots:["shoes"]},
 {kind:"wearable",value:"socks",label:"袜子",slots:["socks"]},
 {kind:"wearable",value:"headwear",label:"帽子 / 头饰",slots:["head"]},
 {kind:"wearable",value:"accessory",label:"配饰",slots:["accessory"]},
 {kind:"wearable",value:"other",label:"其他服饰",slots:wardrobeSlots.map(item=>item.value)},
 {kind:"object",value:"book",label:"书籍",slots:[]},
 {kind:"object",value:"art_supply",label:"绘画 / 文具用品",slots:[]},
 {kind:"object",value:"cup",label:"杯子 / 餐具",slots:[]},
 {kind:"object",value:"electronics",label:"电子设备",slots:[]},
 {kind:"object",value:"tool",label:"工具",slots:[]},
 {kind:"object",value:"personal_item",label:"日常用品",slots:[]},
 {kind:"object",value:"other_object",label:"其他物品",slots:[]},
];
export const wardrobeOwnershipOptions = [{value:"owned",label:"已拥有"},{value:"borrowed",label:"借用"},{value:"unknown",label:"所有权未确认"}];
export const wardrobeAvailabilityOptions = [{value:"available",label:"可用"},{value:"unavailable",label:"不可用"},{value:"lost",label:"已遗失"}];
export function newWardrobeItemDraft(): WardrobeItemDraft { return {itemKind:"wearable",category:"top",slot:"upper_body",description:"",ownership:"owned",availability:"available"}; }
export function wardrobeCategoryOptions(kind:string) {return wardrobeCategories.filter(item=>item.kind===kind);}
export function wardrobeSlotOptions(kind:string,category:string) {
 if(kind==="object") return [{value:"",label:"不适用（普通物品）"}];
 const selected=wardrobeCategories.find(item=>item.kind===kind&&item.value===category);
 return wardrobeSlots.filter(item=>selected?.slots.includes(item.value));
}
export function wardrobeItemPayload(draft:WardrobeItemDraft):Record<string,unknown> {
 if(!wardrobeCategoryOptions(draft.itemKind).some(item=>item.value===draft.category) || !wardrobeSlotOptions(draft.itemKind,draft.category).some(item=>item.value===draft.slot)) throw new Error("请选择匹配的分类和部位。");
 if(!wardrobeOwnershipOptions.some(item=>item.value===draft.ownership)||!wardrobeAvailabilityOptions.some(item=>item.value===draft.availability)) throw new Error("请选择有效的所有权和可用状态。");
 const description=draft.description.trim();if(!description || [...description].length>512) throw new Error("请填写物品描述（最多512字）。");
 return {item_kind:draft.itemKind,category:draft.category,slot:draft.slot,description,ownership:draft.ownership,availability:draft.availability};
}
export function wardrobeCategoryLabel(value:unknown):string {const aliases:Record<string,string>={clothing:"服装",shirt:"上装",blouse:"上装",sweater:"上装",pants:"下装",skirt:"下装",outerwear:"外套"};return wardrobeCategories.find(item=>item.value===value)?.label ?? aliases[String(value)] ?? String(value??"未分类");}
export function wardrobeSlotLabel(value:unknown):string {const aliases:Record<string,string>={top:"上装",bottom:"下装",outerwear:"外套",feet:"鞋履",neck:"颈部配饰",wrist:"腕部配饰",hands:"手部配饰",waist:"腰部配饰"};return wardrobeSlots.find(item=>item.value===value)?.label ?? aliases[String(value)] ?? (value ? String(value) : "不适用");}
export function wardrobeOwnershipLabel(value:unknown):string {return wardrobeOwnershipOptions.find(item=>item.value===value)?.label ?? "所有权未确认";}
export function wardrobeAvailabilityLabel(value:unknown):string {return wardrobeAvailabilityOptions.find(item=>item.value===value)?.label ?? "状态未确认";}

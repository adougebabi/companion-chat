import assert from "node:assert/strict";
import test from "node:test";
import { wardrobeCategoryLabel,wardrobeItemPayload,newWardrobeItemDraft,wardrobeSlotOptions,wardrobeOwnershipOptions,wardrobeAvailabilityOptions } from "../src/lib/wardrobe-form.ts";

test("Chinese wardrobe choices submit canonical states and compatible slots",()=>{
 const draft=newWardrobeItemDraft();draft.category="boots";draft.slot="shoes";draft.description="黑色短靴";draft.ownership="borrowed";
 assert.deepEqual(wardrobeItemPayload(draft),{item_kind:"wearable",category:"boots",slot:"shoes",description:"黑色短靴",ownership:"borrowed",availability:"available"});
 assert.deepEqual(wardrobeSlotOptions("wearable","boots"),[{value:"shoes",label:"鞋履"}]);
 assert.throws(()=>wardrobeItemPayload({...draft,slot:"upper_body"}),/匹配/);
 assert.throws(()=>wardrobeItemPayload({...draft,availability:"stored"}),/有效/);
 assert.deepEqual(wardrobeOwnershipOptions.map(x=>x.value),["owned","borrowed","unknown"]);
 assert.deepEqual(wardrobeAvailabilityOptions.map(x=>x.value),["available","unavailable","lost"]);
});

test("ordinary objects have no wearing slot and additions do not imply wearing",()=>{
 const draft={...newWardrobeItemDraft(),itemKind:"object",category:"art_supply",slot:"",description:"细头画笔"};
 const payload=wardrobeItemPayload(draft);
 assert.equal(payload.slot,"");assert.equal(payload.item_kind,"object");assert.equal(payload.worn,undefined);
 assert.throws(()=>wardrobeItemPayload({...draft,slot:"shoes"}),/匹配/);
});


test("category labels preserve known and Chinese names without exposing unknown keys",()=>{
 assert.equal(wardrobeCategoryLabel("personal"),"日常用品");
 assert.equal(wardrobeCategoryLabel("personal_item"),"日常用品");
 assert.equal(wardrobeCategoryLabel("outdoor_clothing"),"户外服装");
 assert.equal(wardrobeCategoryLabel("书籍与文具"),"书籍与文具");
 for(const value of ["unmapped_category","__proto__","constructor","unknown_物品",42])assert.equal(wardrobeCategoryLabel(value),"其他物品");
 assert.equal(wardrobeCategoryLabel(null),"未分类");
});

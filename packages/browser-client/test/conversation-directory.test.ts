import assert from "node:assert/strict";
import test from "node:test";
import {BrowserClient} from "../src/index.ts";

test("direct directory, persistent selection, full search and exact source use explicit contracts",async()=>{
 const calls:Array<{url:URL;method:string;body:unknown}>=[];
 const client=new BrowserClient("http://fluctlight.local",async(input,init)=>{
  calls.push({url:new URL(String(input)),method:init?.method??"GET",body:init?.body?JSON.parse(String(init.body)):null});
  return Response.json({items:[],nextCursor:"",currentConversationId:"current",conversation:{id:"new-session"},participants:[],messages:[]});
 });
 await client.conversationDirectory("persona/a",{limit:2,cursor:"cursor+1"});
 await client.newDirectConversation("persona/a",{title:"会话",idempotencyKey:"new-1"});
 await client.selectDirectConversation("persona/a","conversation/b");
 await client.searchConversationMessages("persona/a","过去 & 现在",{limit:3,cursor:"search+2"});
 await client.conversationMessage("conversation/b","message/c");
 assert.equal(calls[0]?.url.pathname,"/api/fluctlights/persona%2Fa/conversations");
 assert.equal(calls[0]?.url.searchParams.get("cursor"),"cursor+1");
 assert.deepEqual(calls[1]?.body,{title:"会话",idempotencyKey:"new-1"});
 assert.equal(calls[2]?.method,"POST");
 assert.equal(calls[2]?.url.pathname,"/api/fluctlights/persona%2Fa/conversations/conversation%2Fb/select");
 assert.equal(calls[3]?.url.searchParams.get("q"),"过去 & 现在");
 assert.equal(calls[3]?.url.searchParams.get("cursor"),"search+2");
 assert.equal(calls[4]?.url.pathname,"/api/conversations/conversation%2Fb/messages/message%2Fc");
});

import assert from "node:assert/strict";
import test from "node:test";
import { BrowserClient } from "../src/index.ts";

test("Kev client preserves filter/cursor identity and raw per-question response",async()=>{
  let requested="";
  const raw='{"answers":{"q_0001":{"choice":"yes","probabilities":{"yes":0.6,"no":0.2,"unclear":0.2}}}}';
  const client=new BrowserClient("http://fluctlight.local",async(input)=>{requested=String(input);return Response.json({items:[{id:"decision",request_id:"request",question_id:"q_0001",raw_response:raw}],next_cursor:"next+/=",snapshot:"1:2:"});});
  const page=await client.kevDecisions({actor_self:"actor",decision_point:"persona.switch",cursor:"opaque+/="});
  const url=new URL(requested);assert.equal(url.pathname,"/api/diagnostics/kev-decisions");assert.equal(url.searchParams.get("cursor"),"opaque+/=");assert.equal(url.searchParams.get("decision_point"),"persona.switch");assert.equal(page.items[0]?.raw_response,raw);assert.equal(page.next_cursor,"next+/=");
});

test("Kev connection test uses an explicit mutation without business data",async()=>{
  let requested="";let method="";let body:unknown;
  const client=new BrowserClient("http://fluctlight.local",async(input,init)=>{requested=String(input);method=String(init?.method);body=JSON.parse(String(init?.body));return Response.json({call_status:"ok"});});
  await client.testKevConnection();assert.equal(requested,"http://fluctlight.local/api/settings/kev/test-connection");assert.equal(method,"POST");assert.deepEqual(body,{});
});

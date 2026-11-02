import assert from 'node:assert/strict';
import test from 'node:test';
import {BrowserClient} from '../src/index.ts';

test('wardrobe recovery preserves original command identity and encoded character scope',async()=>{
 const calls:Array<{url:string;method:string;body:unknown}>=[];
 const result={intent_id:'original',workflow_id:'original',status:'pending',restarted:true};
 const client=new BrowserClient('https://fixture.invalid',async(input,init)=>{calls.push({url:String(input),method:init?.method||'GET',body:JSON.parse(String(init?.body))});return Response.json(result);});
 const body={reason:'模型已恢复',idempotencyKey:'original-command'};
 assert.deepEqual(await client.recoverInitialWardrobe('role/one',body),result);
 assert.deepEqual(calls,[{url:'https://fixture.invalid/api/fluctlights/role%2Fone/wardrobe/recover',method:'POST',body}]);
});

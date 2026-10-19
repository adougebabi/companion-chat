import {test} from 'node:test';
import assert from 'node:assert/strict';
import {beginDraftSubmission,settleDraftSubmission} from '../src/v3/proto/components/draft-submission.js';

test('successful save acknowledges its captured note, leaving later edits dirty',()=>{
 const draft={note:'A',favorite:false};
 const submitted=beginDraftSubmission(draft,'note',{note:draft.note,favorite:draft.favorite});
 draft.note='B';draft.favorite=true;
 const result=settleDraftSubmission(draft,submitted,{note:draft.note,favorite:draft.favorite});
 assert.equal(result.unchanged,false);assert.equal(submitted.value.note,'A');assert.equal(draft.note,'B');assert.equal(draft.favorite,true);assert.equal(draft.pending.note,undefined);
});
test('reply acceptance cannot clear text typed after the submitted snapshot',()=>{
 const draft={reply:'first reply'};
 const submitted=beginDraftSubmission(draft,'reply',draft.reply);
 draft.reply='next unsent reply';
 const result=settleDraftSubmission(draft,submitted,draft.reply);if(result.unchanged)draft.reply='';
 assert.equal(draft.reply,'next unsent reply');
 const unchanged=beginDraftSubmission(draft,'reply',draft.reply);
 if(settleDraftSubmission(draft,unchanged,draft.reply).unchanged)draft.reply='';assert.equal(draft.reply,'');
});
test('stable draft identity prevents a second in-flight submission across renderers',()=>{
 const map=new Map([['owner/fid/diary',{note:'A'}]]),first=map.get('owner/fid/diary');
 const pending=beginDraftSubmission(first,'note',{note:first.note});
 const nextRenderer=map.get('owner/fid/diary');assert.equal(nextRenderer,first);
 assert.throws(()=>beginDraftSubmission(nextRenderer,'note',{note:'duplicate'}),/仍在处理中/);
 assert.equal(settleDraftSubmission(nextRenderer,pending,{note:'A'}).unchanged,true);
 assert.equal(settleDraftSubmission(nextRenderer,pending,{note:'A'}),null);
});
test('governance reopen preserves edits while sharing the submitted version and pending state',()=>{
 const drafts=new Map([['owner/fid/proposal',{base:3,reason:'A',changes:'{"identity":{"name":"A"}}',pending:{}}]]);
 const original=drafts.get('owner/fid/proposal');const snapshot=()=>({base:original.base,reason:original.reason,changes:original.changes});
 const request=beginDraftSubmission(original,'proposal',snapshot());
 const reopened=drafts.get('owner/fid/proposal');reopened.reason='B';
 assert.throws(()=>beginDraftSubmission(reopened,'proposal',snapshot()),/仍在处理中/);
 const acknowledged=settleDraftSubmission(original,request,snapshot());if(acknowledged.unchanged)drafts.delete('owner/fid/proposal');
 assert.equal(drafts.get('owner/fid/proposal').reason,'B');assert.equal(request.value.reason,'A');assert.equal(reopened.base,3);
});

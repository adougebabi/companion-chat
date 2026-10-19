// Only presentation drafts live here. Capture the submitted value so a late
// success cannot acknowledge or clear edits made while the request was pending.
export function beginDraftSubmission(draft,channel,value){
 draft.pending ||= {};
 if(draft.pending[channel])throw new Error('这次提交仍在处理中，请等待实际结果');
 const submission={channel,value:structuredClone(value)};
 draft.pending[channel]=submission;
 return submission;
}
export function settleDraftSubmission(draft,submission,current){
 if(draft.pending?.[submission.channel]!==submission)return null;
 delete draft.pending[submission.channel];
 return {unchanged:JSON.stringify(current)===JSON.stringify(submission.value)};
}

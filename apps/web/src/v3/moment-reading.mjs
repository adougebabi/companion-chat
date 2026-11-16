import {momentBodyText} from '../lib/public-body.mjs';
// Explicit reading projection for the observed historical corruption. Stored
// prose stays immutable; this only shapes what the page shows.
// Character moments also drop model preambles, Markdown wrappers and trailing
// generation notes. Every visible layer (list, detail, folded technical record
// and its download) uses this projection, so the wrappers never reach users.
export function momentReading(post){
 const original=post.author_type==='fluctlight'?momentBodyText(post.text):String(post.text||'');
 const tail=post.author_type==='fluctlight'&&!/古吉拉特|Gujarati/i.test(original)&&/[\u3400-\u9fff]/u.test(original)?original.match(/[。！？](\p{Script=Gujarati}{1,12})\s*$/u):null;
 return {text:tail?original.slice(0,tail.index+1):original,original:tail?original:'',corrected:!!tail};
}

// Technical-record projection of a moment: same fields as the stored post, but
// the visible body is the cleaned reading text shown on the first screen.
export function momentRecord(post){
 if(!post||typeof post!=='object')return post;
 return {...post,text:momentReading(post).text};
}

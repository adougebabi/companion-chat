import {momentBodyText} from '../lib/public-body.mjs';
// Explicit reading projection for the observed historical corruption. Stored
// prose stays immutable and the original is always available beside this view.
// Character moments also drop model preambles, Markdown wrappers and trailing
// generation notes; that raw text stays in the technical record only.
export function momentReading(post){
 const original=post.author_type==='fluctlight'?momentBodyText(post.text):String(post.text||'');
 const tail=post.author_type==='fluctlight'&&!/古吉拉特|Gujarati/i.test(original)&&/[\u3400-\u9fff]/u.test(original)?original.match(/[。！？](\p{Script=Gujarati}{1,12})\s*$/u):null;
 return {text:tail?original.slice(0,tail.index+1):original,original:tail?original:'',corrected:!!tail};
}

import {esc,badge} from '../ui.js';
import {attributes} from './forms.js';

// Slots are markup produced by the shared form/revision components. Both the
// frozen Mock baseline and the real data adapter use this same card structure.
export function AboutYouCard({subtitle,fields,advanced='',conflict='',actions,result='',formAttrs={}}){
 return `<form class="card" style="max-width:640px" ${attributes(formAttrs)}><div class="row"><img class="av lg" src="/v3/assets/avatars/user.svg" alt=""><div class="grow"><h3 style="margin:0">关于你</h3><p class="small muted">${esc(subtitle)}</p></div>${badge('角色范围')}</div>${conflict}<div class="stack">${fields}</div>${advanced}<div class="row" style="margin-top:12px">${actions}</div>${result}</form>`;
}

import {Tabs} from './forms.js';
export function GovernanceTabs(fid,value){return Tabs({value,items:[['capabilities','能力申请'],['foundation','身份核心与提议'],['developing','发展中的自我']].map(([value,label])=>({value,label,href:`#/c/${encodeURIComponent(fid)}/governance?tab=${value}`}))});}

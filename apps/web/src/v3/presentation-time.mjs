import {resolveTimezone} from '../lib/fluctlight-display.ts';
import {formatInstantInZone} from '../lib/instant.ts';

export function displayTimezone(...zones){
 const device=Intl.DateTimeFormat().resolvedOptions().timeZone;
 return resolveTimezone(...zones.filter(z=>typeof z==='string'&&z.trim()),device);
}
export function userInstant(value,zone){
 if(!value)return '未记录';
 if(typeof value!=='string'||!Number.isFinite(new Date(value).getTime()))return '时间未知';
 const chosen=displayTimezone(zone);
 return formatInstantInZone(value,chosen).replace('T',' ').replace(/([+-]\d{2}:\d{2})$/,' $1')+`（${chosen}）`;
}

import {resolveTimezone} from '../lib/fluctlight-display.ts';
import {formatBeijingInstant} from '../lib/instant.ts';

export function displayTimezone(...zones){
 const device=Intl.DateTimeFormat().resolvedOptions().timeZone;
 return resolveTimezone(...zones.filter(z=>typeof z==='string'&&z.trim()),device);
}
// Zone arguments remain accepted for existing callers; reading uses Beijing time.
export function userInstant(value,_zone){return formatBeijingInstant(value);}
export function readingInstant(value,_zone){return formatBeijingInstant(value);}
// Compare instants, never locale strings. A boundary belongs only to the next slot.
export function planWindow(start,end,now=Date.now()){
 const from=typeof start==='string'?Date.parse(start):NaN,to=typeof end==='string'?Date.parse(end):NaN;
 if(!Number.isFinite(from)||!Number.isFinite(to)||!Number.isFinite(now)||to<=from)return 'unknown';
 return now<from?'future':now<to?'current':'past';
}

import {lifeSections} from '../../workspace-navigation.mjs';
import {Tabs} from './forms.js';

export function LifeNavigation(ctx){
 const base=`/c/${ctx.charId}/`;
 return Tabs({className:'tabs module-navigation',attrs:{'aria-label':'生活页面'},value:ctx.path,items:lifeSections.map(([label,path])=>({value:base+path,label,href:'#'+base+path}))});
}

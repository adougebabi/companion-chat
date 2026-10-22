import { lifeClient } from '../services.js';
import { asyncPage } from '../components/async-page.js';
import { esc,icon,badge } from '../ui.js';
import { IntentionList } from '../components/cards.js';
import { JsonDetails } from '../components/records.js';
import {displayTimezone} from '../../presentation-time.mjs';
import {stateLabel} from '../../domain-presentation.mjs';
import {ScheduleTimeRange} from '../components/schedule-records.js';
import {WearingSummary} from '../components/inventory-records.js';
import {Avatar,bindAvatars} from '../components/avatar.js';

export function renderRealNow(el,ctx) {
  return asyncPage(el,()=>lifeClient.now(ctx.charId),({detail,wardrobe,nearby},reload)=>{
    const c=ctx.store.get('characters',ctx.charId), current=detail.context, appearance=detail.current_state?.appearance;
    if(!current || !appearance)throw new Error('当前生活投影缺失，不能判断此刻状态');
    const plan=detail.schedule,zone=displayTimezone(plan?.timezone,current.timezone);
    const worn=appearance.worn_items || [], intentions=detail.intentions || [];
    const upcoming=(plan?.items||[]).filter(item=>Date.parse(item.start_at)>Date.now()).slice(0,3);
	const mood=detail.inner_state?.mood, moodLabel=typeof mood==='string'?mood:mood?.label;
	if(moodLabel!=null&&typeof moodLabel!=='string')throw new Error('心情投影格式无效，请查看运行诊断');
	const drives=detail.inner_state?.drives;
	if(drives!=null&&!Array.isArray(drives))throw new Error('驱动投影格式无效，请查看运行诊断');
    el.innerHTML=`<h2 class="page-title">此刻</h2><section class="hero now-summary" data-test="now-hero"><div class="over">${Avatar({src:c.avatar,name:c.name})}<div class="grow"><div class="small muted">${esc(c.name)} · 时间以北京时间显示</div><h2 data-test="now-doing">${esc(current.activity || '此刻活动尚未确认')}</h2><div class="row small">${badge(current.source==='event'&&current.authority_status==='confirmed'?'已确认':current.source==='schedule'?'日程预计':'未知')}${icon('pin')}${esc(current.location || '地点未知')}<span class="muted">· 远程查看，不会移动她</span></div></div><button class="btn sm" data-now-refresh>重新读取</button></div></section>
    <div class="grid g3 now-overview" style="margin-top:16px"><div class="card"><h3>${icon('calendar')} 接下来的安排</h3>${plan?upcoming.map(i=>`<div class="lrow next-plan-row"><span class="small muted">${ScheduleTimeRange(i.start_at,i.end_at,zone)}</span><div class="row"><b class="grow">${esc(i.activity)}</b>${i.status?badge(stateLabel(i.status)):''}</div></div>`).join('')||'<p class="small muted">今天后续无计划</p>':'<p class="small muted">今天尚无已接受日程</p>'}<a class="small" href="#/c/${ctx.charId}/life/schedule">完整日程 →</a></div>
    <div class="card tint-sun"><h3>${icon('heart')} 心情 / 精力</h3><p>心情：${esc(moodLabel?.trim() || '尚未记录')}</p><p>驱动状态：${drives?.length?`已记录 ${drives.length} 项`:'尚无驱动记录'}</p>${drives?.length?JsonDetails(drives,'查看驱动记录'):''}<p class="small muted">内在状态来源，非外部事实</p></div>
    <div class="card tint-teal"><h3>${icon('shirt')} 当前穿着 ${badge(appearance.wearing_state==='known'?'已确认':'未知')}</h3>${WearingSummary(worn,{known:appearance.wearing_state==='known',link:i=>`#/c/${encodeURIComponent(ctx.charId)}/life/wardrobe?item=${encodeURIComponent(i.item_id||i.id||'')}`})}<a class="small" href="#/c/${ctx.charId}/life/wardrobe">打开衣柜 →</a><details class="small muted"><summary>穿着来源与版本</summary><p>穿着版本 ${esc(wardrobe.revision)}</p></details></div></div>
    <div class="grid g2" style="margin-top:16px"><div class="card"><h3>${icon('target')} 待兑现的约定与意向</h3>${IntentionList(intentions,{fid:ctx.charId,goals:detail.goals||[],profiles:detail.core_persona?.personality_system?.profiles||[]})}<a href="#/c/${ctx.charId}/life/goals">目标与约定 →</a></div><div class="card"><h3>${icon('people')} 此地附近</h3>${nearby.actors.map(n=>`<div class="lrow"><a href="#/c/${ctx.charId}/people/npc/${n.npc_id}">${esc(n.name)}</a>${badge(n.presence==='confirmed'?'已确认':'预计在场')}</div>`).join('')||`<p class="small muted">${nearby.location_known?'当前地点未读取到在场人物':'地点未知，无法判断附近人物'}</p>`}<a href="#/c/${ctx.charId}/world">远程查看场景 →</a></div></div>`;
    bindAvatars(el);
    el.querySelector('[data-now-refresh]').onclick=reload;
  });
}

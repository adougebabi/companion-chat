// Reading projection only. A planned date never becomes proof of execution.
export function lifeTimelineRows(fid,sources){
 const base='#/c/'+encodeURIComponent(fid),rows=[];
 const add=(kind,id,at,title,text,href,extra={})=>rows.push({id:kind+':'+id,kind,at,title,text,href,...extra});
 for(const day of sources.schedule||[])for(const item of day.schedule?.items||[])add('schedule',item.id,item.start_at,item.activity||'今天的安排',item.location||'',base+'/life/schedule?date='+encodeURIComponent(day.local_date),{label:'日程安排'});
 for(const d of sources.diary||[])if(typeof d.body==='string'&&d.body.trim())add('diary',d.id,d.date,'她写下的这一天',d.body,base+'/life/diary/'+encodeURIComponent(d.id),{label:'日记',localDay:true});
 for(const post of sources.moments||[])if(post.author_type==='fluctlight')add('moments',post.id,post.created_at,'她的动态',post.text,'#/feed/'+encodeURIComponent(post.id),{label:'动态'});
 for(const asset of sources.media||[])if(asset.kind==='image'&&asset.readable)add('media',asset.id,asset.created_at,asset.title||'相册里的图片','',base+'/media/'+encodeURIComponent(asset.id),{label:'图片',imageId:asset.id});
 for(const goal of sources.goals||[])add('goals',goal.id,goal.updated_at||goal.created_at,'她最近在努力的事',goal.desired_outcome||goal.title,base+'/life/goal/'+encodeURIComponent(goal.id),{label:'目标'});
 return rows.sort((a,b)=>(Date.parse(b.at)||0)-(Date.parse(a.at)||0)||a.id.localeCompare(b.id));
}

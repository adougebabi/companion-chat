import {
  domainName,
  errorMessage,
  stateLabel,
} from "../v3/domain-presentation.mjs";
export const joined = (...values) =>
  values.filter((value) => value != null && String(value).trim()).join(" · ");
export function readablePrompt(value) {
  let record=value;
  if(typeof record==='string'){
    try{const parsed=JSON.parse(record);if(Array.isArray(parsed?.messages)||Array.isArray(parsed))record=parsed;else return record;}catch{return record;}
  }
  const messages=Array.isArray(record)?record:Array.isArray(record?.messages)?record.messages:null;
  if(!messages)return typeof record?.content==='string'?record.content:JSON.stringify(record??{},null,2);
  const content=(part)=>typeof part==='string'?part:part?.type==='text'?part.text:part?.type==='image_url'?'（图片输入，原始结构见技术详情）':JSON.stringify(part,null,2);
  return messages.map(message=>`[${message.role||'记录'}]\n${Array.isArray(message.content)?message.content.map(content).join('\n'):content(message.content??'')}${message.tool_calls?.length?'\n工具调用：\n'+JSON.stringify(message.tool_calls,null,2):''}`).join('\n\n');
}
export function uniqueReplyNotifications(rows){
  const seen=new Set();
  return rows.filter(row=>{
    const key=row.id;
    if(seen.has(key))return false;seen.add(key);return true;
  });
}
export function notificationGroups(rows,characters,includeRead=false){
  const groups=new Map();
  for(const notice of uniqueReplyNotifications(rows.filter(n=>includeRead||!n.read_at))){
    const key=notice.fluctlight_id||'system';
    if(!groups.has(key)){
      const character=characters.find(c=>c.id===notice.fluctlight_id);
      groups.set(key,{id:key,name:character?.name||(notice.fluctlight_id?'角色名称未读取':'系统通知'),initial:character?.initial||'光',color:character?.color||'#e4ede4',notices:[]});
    }
    groups.get(key).notices.push(notice);
  }
  return [...groups.values()];
}
export function notificationPath(link){
  const path=typeof link==='string'?link.replace(/^#/,''):'';
  return path.startsWith('/')&&!path.startsWith('//')&&!/[\\\r\n]/.test(path)?path:null;
}
export function notificationIssueGroups(rows){
  const groups=new Map();
  for(const row of rows){
    const fid=row.fid||row._pending?.fluctlight_id||row._record?.data?.metrics?.fluctlight_id||row._record?.data?.fluctlight_id||'';
    const key=JSON.stringify([fid,row.code||row._pending?.intent_type||'',row.label]);
    if(!groups.has(key))groups.set(key,{id:key,label:row.label,items:[]});
    const group=groups.get(key);
    if(!group.items.some(item=>item.id===row.id))group.items.push(row);
  }
  return [...groups.values()];
}
export function scheduleProjection(item, schedule, format) {
  const activity =
      typeof item.activity === "string" ? item.activity.trim() : "",
    unspecified = !activity || activity === "未安排时间";
  return {
    time: unspecified ? "" : format(item.start_at, schedule?.timezone),
    text: unspecified ? "尚未安排具体活动" : activity,
    editable:
      !!item.id && !unspecified && Number.isFinite(Date.parse(item.start_at)),
    _source: item,
    _schedule: schedule,
  };
}
export function nextScheduleItem(items, now = Date.now()) {
  return items.filter(item => {
    const source=item._source;
    return source && Number.isFinite(Date.parse(source.start_at)) &&
      Date.parse(source.start_at)>now &&
      !['completed','cancelled','failed','in_progress'].includes(source.action_status) &&
      !['cancelled','rejected'].includes(source.status);
  }).sort((a,b)=>Date.parse(a._source.start_at)-Date.parse(b._source.start_at))[0];
}
export function relativeTime(value, zone = 'Asia/Shanghai', now = Date.now()) {
  const at=typeof value==='string'?Date.parse(value):NaN;
  if(!Number.isFinite(at))return '';
  const age=now-at;
  if(age<0)return new Intl.DateTimeFormat('zh-CN',{timeZone:zone,dateStyle:'short',timeStyle:'short'}).format(at);
  if(age<60000)return '刚刚';
  if(age<3600000)return Math.floor(age/60000)+' 分钟前';
  if(age<86400000)return Math.floor(age/3600000)+' 小时前';
  const parts=Object.fromEntries(new Intl.DateTimeFormat('en-CA',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit'}).formatToParts(now).map(p=>[p.type,p.value]));
  const previous=new Date(Date.UTC(Number(parts.year),Number(parts.month)-1,Number(parts.day))-86400000).toISOString().slice(0,10);
  const date=new Intl.DateTimeFormat('en-CA',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit'}).format(at);
  return date===previous?'昨天':date;
}
// Compact list copy keeps the original instant in the element's title/datetime.
export function compactDateTime(value, zone = 'Asia/Shanghai', now = Date.now(), relativeDay = false) {
  const at=typeof value==='string'?Date.parse(value):NaN;
  if(!Number.isFinite(at))return '';
  const parts=time=>Object.fromEntries(new Intl.DateTimeFormat('en-CA',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'}).formatToParts(time).map(p=>[p.type,p.value]));
  const p=parts(at),today=parts(now),date=p.year+'-'+p.month+'-'+p.day;
  const yesterday=new Date(Date.UTC(Number(today.year),Number(today.month)-1,Number(today.day))-86400000).toISOString().slice(0,10);
  const label=relativeDay?(date===today.year+'-'+today.month+'-'+today.day?'今天':date===yesterday?'昨天':p.month+'/'+p.day):p.month+'/'+p.day;
  return label+(relativeDay?' · ':' ')+p.hour+':'+p.minute;
}
export function readableSummary(
  value,
  fallback = "结构化记录，详情中可查看完整内容",
  depth = 0,
) {
  if (depth > 5 || value == null) return fallback;
  if (typeof value === "string") {
    const text = value.trim();
    if (!text) return fallback;
    if(/(^|\n)\s*(?:```|#{1,6}\s|\|)/.test(text))return fallback;
    if (/^[\[{]/.test(text)) {
      try {
        return readableSummary(JSON.parse(text), fallback, depth + 1);
      } catch {
        return fallback;
      }
    }
    return /[\u3400-\u9fff]/.test(text)?text.replace(/\s+/g, " ").slice(0, 180):fallback;
  }
  if (Array.isArray(value)) {
    const text = value
      .slice(-4)
      .map((v) => readableSummary(v, "", depth + 1))
      .filter(Boolean)
      .join("；");
    return text || fallback;
  }
  if (typeof value === "object") {
    for (const key of [
      "summary",
      "text",
      "message",
      "content",
      "messages",
      "items",
      "result",
    ])
      if (value[key] != null) {
        const text = readableSummary(value[key], "", depth + 1);
        if (text) return text;
      }
  }
  return fallback;
}
export function diagnosticProjection(
  record,
  index,
  characters,
  now = Date.now(),
) {
  const d = record.data || {},
    scenario=d.scenario||d.trigger_scenario||d.metrics?.scenario||d.metrics?.trigger_scenario||d.role,
    fid = d.fluctlight_id || d.metrics?.fluctlight_id,
    character =
      characters.find((c) => c.id === fid)?.name ||
      (fid ? "角色名称未读取" : ['initialization','persona_compilation'].includes(scenario)?"创建中的人物":"未记录角色来源"),
    start = Date.parse(d.started_at || d.queued_at || record.occurred_at) || 0,
    queued = Date.parse(d.queued_at) || start,
    completed = Date.parse(d.completed_at || ""),
    pending = ["queued", "running", "pending", "started"].includes(d.status),
    status = ["failed", "cancelled", "timeout", "dead_letter"].includes(
      d.status,
    )
      ? "error"
      : pending
        ? "pending"
        : ["completed", "ready", "succeeded"].includes(d.status)
          ? "success"
          : "unknown",
    label = domainName(
      scenario || d.binding_role,
      index ? "媒体生成" : "模型调用",
    );
  const rawInput = readablePrompt(d.prompt),
    rawResponse =
      typeof d.response === "string"
        ? d.response
        : d.response==null?"":JSON.stringify(d.response, null, 2),
    latency = Number(d.latency_ms ?? d.metrics?.latency_ms),
    duration =
      Number.isFinite(latency) && latency >= 0
        ? Math.round(latency / 1000)
        : Number.isFinite(completed) && start
          ? Math.max(0, Math.round((completed - start) / 1000))
          : null,
    elapsed = queued ? Math.max(0, Math.floor((now - queued) / 1000)) : null;
  return {
    id: record.ref,
    kind: index ? "媒体调用" : "模型调用",
    label,
    character,
    status,
    start,
    duration,
    durationLabel: pending
      ? elapsed == null
        ? "等待时间待确认"
        : `已等待 ${elapsed} 秒`
      : duration == null
        ? "耗时未记录"
        : `${duration} 秒`,
    input: rawInput,
    response: rawResponse,
    summary:
      status === "error"
        ? errorMessage(
            { code: d.error_code, message: d.error },
            "本次调用未完成，可查看原记录。",
          )
        : pending
          ? joined(
              stateLabel(d.status),
              elapsed == null ? "" : `已等待 ${elapsed} 秒`,
            )
          : label + "已完成，详情中可查看实际返回内容。",
    reason: d.error_code
      ? errorMessage({ code: d.error_code }, "本次调用未完成，可查看原记录。")
      : "",
    category: d.error_code ? "执行失败" : "",
    code: d.error_code || "",
    correlationId: d.correlation_id || record.ref,
    model: d.model_id || "",
    _record: record,
    media: index
      ? {
          type:
            { image: "图片", audio: "语音", video: "视频" }[d.kind] || "图片",
          workflow: d.workflow_id || "",
          prompt:
            typeof d.provider_prompt === "string"
              ? d.provider_prompt
              : JSON.stringify(d.provider_prompt || d.prompt || {}, null, 2),
          negativePrompt: d.negative_prompt || "",
          params: d.parameters || {},
          output: readableSummary(d.result, "尚未记录可读输出摘要"),
          assetId: d.result?.asset_id,
        }
      : null,
  };
}
export function pendingIssues(rows, characters) {
  const result = new Map();
  for (const row of rows) {
    const kind = row.intent_type || row.title || row.kind,
      key =
        kind === "wardrobe.initialize"
          ? kind + ":" + row.fluctlight_id
          : row.source_id || row.id,
      previous = result.get(key);
    if (
      previous &&
      Date.parse(previous._pending.created_at || 0) >
        Date.parse(row.created_at || 0)
    )
      continue;
    const name = characters.find((c) => c.id === row.fluctlight_id)?.name;
    result.set(key, {
      id: row.id,
      label: joined(name, domainName(kind, "待处理事项")),
      reason: joined(
        stateLabel(row.status),
        row.status === "failed" || row.status === "dead_letter"
          ? "请查看原任务并恢复"
          : "等待原任务继续处理",
      ),
      _pending: row,
    });
  }
  return [...result.values()];
}

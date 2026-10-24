export function previousDiaryDate(today) {
  const time=Date.parse(today+'T00:00:00Z');
  if(!Number.isFinite(time))return '';
  return new Date(time-86400000).toISOString().slice(0,10);
}
export function diaryRequestText(receipt) {
  const date=receipt?.local_date||'这一天';
  return ({pending:`${date} 的日记已排队。`,running:`正在回顾 ${date} 的真实经历。`,completed:`${date} 的日记已生成。`,no_content:`${date} 没有足够或适合的素材形成日记。`,failed:`${date} 的日记没有生成，可查看原任务并重试。`,cancelled:`${date} 的日记回顾已停止。`})[receipt?.status]||'正在核对原日记请求的状态。';
}

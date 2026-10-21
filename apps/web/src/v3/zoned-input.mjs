// Wall-clock inputs use the schedule's zone, never the device's implicit zone.
function partsAt(value, zone) {
  return Object.fromEntries(new Intl.DateTimeFormat('en-CA', {timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'}).formatToParts(new Date(value)).map(part=>[part.type,part.value]));
}
export function zonedInput(value, zone) {
  if (!value || !Number.isFinite(Date.parse(value))) return '';
  const p=partsAt(value,zone);
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`;
}
export function zonedInputInstant(value, zone, original='') {
  if (original && zonedInput(original,zone)===value) return original;
  const match=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value||'');
  if (!match) throw new Error('请选择完整的日期和时间');
  const [,year,month,day,hour,minute]=match.map(Number), wall=Date.UTC(year,month-1,day,hour,minute);
  if (new Date(wall).toISOString().slice(0,16)!==value) throw new Error('日期或时间无效');
  const offsets=new Set();
  for (let hours=-36;hours<=36;hours+=6) {
    const sample=wall+hours*3600000,p=partsAt(sample,zone);
    offsets.add(Date.UTC(Number(p.year),Number(p.month)-1,Number(p.day),Number(p.hour),Number(p.minute),Number(p.second))-sample);
  }
  const candidates=[...offsets].filter(offset=>zonedInput(new Date(wall-offset).toISOString(),zone)===value);
  if (!candidates.length) throw new Error('该时间处于夏令时跳过的时段，请选择另一个时间');
  if (candidates.length>1) throw new Error('该时间在夏令时切换时出现两次，请启用带时区偏移的时间输入');
  const offset=candidates[0]/60000, absolute=Math.abs(offset);
  return `${value}:00${offset<0?'-':'+'}${String(Math.floor(absolute/60)).padStart(2,'0')}:${String(absolute%60).padStart(2,'0')}`;
}

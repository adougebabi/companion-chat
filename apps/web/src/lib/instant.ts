// Instant display uses a chosen IANA zone. It never changes an Actor's location
// or timezone fact; the numeric offset is calculated for this date.
export function formatInstantInZone(value: string, timezone: string): string {
  const date = new Date(value);
  if (!value || !Number.isFinite(date.getTime())) return "时间未知";
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", second: "2-digit", fractionalSecondDigits: 3,
    hourCycle: "h23", timeZoneName: "longOffset",
  }).formatToParts(date);
  const fields = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  const offset = fields.timeZoneName === "GMT" ? "+00:00" : fields.timeZoneName.replace("GMT", "");
  return `${fields.year}-${fields.month}-${fields.day}T${fields.hour}:${fields.minute}:${fields.second}.${fields.fractionalSecond}${offset}`;
}

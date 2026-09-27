// datetime-local has no timezone. Interpret its wall clock in the selected
// Fluctlight's zone and reject DST gaps; folds choose the earlier instant.
export function zonedLocalInputToISO(value: string, timezone: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
  if (!match) throw new Error("local_time_invalid");
  const [year, month, day, hour, minute] = match.slice(1).map(Number);
  const wall = Date.UTC(year, month - 1, day, hour, minute);
  if (!Number.isFinite(wall) || new Date(wall).toISOString().slice(0, 16) !== `${match[1]}-${match[2]}-${match[3]}T${match[4]}:${match[5]}`) {
    throw new Error("local_time_invalid");
  }
  const formatter = new Intl.DateTimeFormat("en-GB", {
    timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  });
  const matches: number[] = [];
  for (let offset = -840; offset <= 840; offset += 15) {
    const candidate = wall - offset * 60_000;
    const parts = Object.fromEntries(formatter.formatToParts(candidate).map((part) => [part.type, part.value]));
    if (Number(parts.year) === year && Number(parts.month) === month && Number(parts.day) === day && Number(parts.hour) === hour && Number(parts.minute) === minute) {
      matches.push(candidate);
    }
  }
  if (!matches.length) throw new Error("local_time_nonexistent");
  return new Date(Math.min(...matches)).toISOString();
}

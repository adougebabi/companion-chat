// Display-only normalization: the stored name and all business text stay intact.
export function avatarInitial(name, initial = "", fallback = "光") {
  const source = String(name || initial || "").trim();
  const display = source.replace(/^FT-\s*/i, "").trim();
  const displayInitial = String(initial || "").replace(/^FT-\s*/i, "").trim();
  if (!/^FT-/i.test(source) && displayInitial) return displayInitial;
  return Array.from(display)[0] || fallback;
}

export function avatarSource(value) {
  return typeof value === "string" ? value : value?.url || "";
}

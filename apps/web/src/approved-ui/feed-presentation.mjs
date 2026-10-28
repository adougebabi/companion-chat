export const momentAuthor = (row) =>
  row.author_type === "user" ? "我" : row.author_name || "作者名称未记录";
export const momentCount = (value) =>
  Number.isSafeInteger(value) && value >= 0 ? String(value) : "—";
export function feedFilters(query) {
  return {
    fid: query?.get("fid") || "",
    author: query?.get("author") === "me" ? "me" : "",
    status: ["visible", "hidden", "all"].includes(query?.get("status"))
      ? query.get("status")
      : "visible",
    q: query?.get("q") || "",
  };
}
export function feedHref(filters) {
  const query = new URLSearchParams({
    ...(filters.fid ? { fid: filters.fid } : {}),
    ...(filters.author ? { author: filters.author } : {}),
    status: filters.status,
    ...(filters.q ? { q: filters.q } : {}),
  });
  return "#/feed?" + query;
}
export function checkMoment(row, id) {
  if (
    !row?.id ||
    (id && row.id !== id) ||
    !Number.isSafeInteger(row.revision) ||
    row.revision < 1 ||
    !["visible", "hidden"].includes(row.status)
  )
    throw Error("动态身份、版本或展示状态尚未完整确认。");
  return row;
}
export function momentText(value) {
  const text = String(value || "").trim();
  if (!text || [...text].length > 2000)
    throw Error("请填写正文，最多2000个字符。");
  return text;
}
export function publishedMomentIds(request) {
  return [
    ...new Set(
      (request?.result?.effects || [])
        .filter(
          (e) =>
            e.output?.target_kind === "moment" &&
            typeof e.output.target_ref === "string" &&
            e.output.target_ref,
        )
        .map((e) => e.output.target_ref),
    ),
  ];
}

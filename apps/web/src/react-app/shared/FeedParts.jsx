import { Button } from "@/components/ui/button";
import { isInternalId } from "@/v3/internal-id.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { ReaderFeedback } from "./Presentation";
import LoadingState from "./LoadingState";
export function feedDisplayText(value) {
  return typeof value === "string" && !isInternalId(value) ? value.trim() : "";
}
export function feedAuthorName(row) {
  if (row.author_type === "user") return "我";
  return row.author_name === row.author_actor_id ? "" : feedDisplayText(row.author_name);
}
export function FeedTime({ value }) {
  if (typeof value !== "string" || !Number.isFinite(Date.parse(value))) return null;
  return <time dateTime={value}>{userInstant(value)}</time>;
}
export function FeedReader({source,label}){
 return <>{source.busy&&!source.ready&&<LoadingState label={'正在读取'+label+'…'}/>}<ReaderFeedback source={source} label={label}/></>;
}
export function FeedMore({ source, label }) {
  return source.cursor ? (
    <Button variant="outline" disabled={source.busy} onClick={source.more}>
      加载更多{label}
    </Button>
  ) : null;
}

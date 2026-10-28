import { Button } from "@/components/ui/button";
import { InlineFeedback } from "./Presentation";
import LoadingState from "./LoadingState";
export function FeedReader({ source, label }) {
  return (
    <>
      {source.busy && <LoadingState label={"正在读取" + label + "…"} />}{" "}
      {source.error && (
        <InlineFeedback role="alert">
          {source.error}
          <Button variant="outline" onClick={source.retry}>
            重新读取{label}
          </Button>
        </InlineFeedback>
      )}
    </>
  );
}
export function FeedMore({ source, label }) {
  return source.cursor ? (
    <Button variant="outline" disabled={source.busy} onClick={source.more}>
      加载更多{label}
    </Button>
  ) : null;
}

import { Button } from "@/components/ui/button";
import { ReaderFeedback } from "./Presentation";
import LoadingState from "./LoadingState";
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

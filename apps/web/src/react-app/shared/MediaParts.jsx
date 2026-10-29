import { useState } from "react";
import { ImageIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { InlineFeedback,ReaderFeedback } from "./Presentation";
import LoadingState from "./LoadingState";
import MediaPreview from "./MediaPreview";
import useWorkspaceRead from "./useWorkspaceRead";
import { mediaReadState } from "@/approved-ui/media-presentation.mjs";
import CursorControls from "./CursorControls";
export function MediaReader({source,label}){
 return <>{source.busy&&!source.ready&&<LoadingState label={'正在读取'+label+'…'}/>}<ReaderFeedback source={source} label={label}/></>;
}
export function MediaMore({ source, label }) {
  return (
    <CursorControls
      label={label}
      append={{
        loading: source.busy,
        hasMore: !!source.cursor,
        loadMore: source.more,
        retry: source.retry,
      }}
    />
  );
}
export function SavedMedia({
  asset,
  model: m,
  className = "",
  thumbnail = false,
}) {
  const [failed, setFailed] = useState(false);
  if (!asset?.readable)
    return (
      <div className={"media-unavailable " + className} role="status">
        <ImageIcon size={25} />
        <p>{asset ? mediaReadState(asset) : "尚未读到素材"}</p>
      </div>
    );
  const src = asset.url || m.service.url(asset.id),
    title = asset.title || asset.source_filename || "媒体素材";
  if (thumbnail)
    return asset.kind === "image" && !failed ? (
      <img
        className={className}
        src={src}
        alt={title}
        loading="lazy"
        onError={() => setFailed(true)}
      />
    ) : (
      <div className={"media-unavailable " + className}>
        <ImageIcon size={25} />
        <span>
          {failed
            ? "预览读取失败"
            : asset.kind === "audio"
              ? "音频素材"
              : "视频素材"}{" "}
          · 打开详情查看
        </span>
      </div>
    );
  if (asset.kind === "image")
    return <MediaPreview src={src} alt={title} className={className} />;
  if (failed)
    return (
      <InlineFeedback role="alert">
        媒体读取失败。
        <Button variant="outline" onClick={() => setFailed(false)}>
          重新读取媒体
        </Button>
      </InlineFeedback>
    );
  if (asset.kind === "audio")
    return (
      <audio
        className={className}
        controls
        preload="metadata"
        src={src}
        aria-label={title}
        onError={() => setFailed(true)}
      />
    );
  if (asset.kind === "video")
    return (
      <video
        className={className}
        controls
        preload="metadata"
        src={src}
        aria-label={title}
        onError={() => setFailed(true)}
      />
    );
  return <p>此素材类型暂不支持预览。</p>;
}
export function MediaById({
  id,
  model: m,
  className = "",
  onOpen,
  label = "查看实际素材",
  expected,
}) {
  const source = useWorkspaceRead({
    identity: [
      "media-image",
      id,
      expected?.asset_revision,
      expected?.version,
      expected?.sha256,
    ],
    characterScoped: false,
    enabled: !!id,
    read: async () => {
      const row = await m.readAsset(id);
      if (
        expected &&
        ((expected.asset_revision !== undefined &&
          row.asset_revision !== expected.asset_revision) ||
          (expected.version && row.version !== expected.version) ||
          (expected.sha256 && row.sha256 !== expected.sha256))
      )
        throw Error("所选图片来源已经变化，请明确重新选择当前版本。");
      return { items: [row], next_cursor: "" };
    },
  });
  return (
    <div className={"media-by-id " + className}>
      <MediaReader source={source} label="实际图片" />
      {source.items[0] && (
        <SavedMedia
          asset={source.items[0]}
          model={m}
          className="media-actual-image"
        />
      )}
      {source.items[0] && onOpen && (
        <Button
          variant="ghost"
          onClick={(event) => onOpen(id, event.currentTarget)}
        >
          {label}
        </Button>
      )}
    </div>
  );
}

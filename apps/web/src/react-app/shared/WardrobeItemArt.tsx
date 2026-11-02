import { useState } from "react";
import GarmentArt from "./WorkspaceGarmentArt";
import { Button } from "@/components/ui/button";

type Props = {
  type: string;
  name?: string;
  media?: Array<{ asset_id: string; created_at?: string }>;
  showStatus?: boolean;
  urlForAsset: (id: string) => string;
  allowRetry?: boolean;
};

export default function WardrobeItemArt({ type, name = "", media, showStatus = false, urlForAsset, allowRetry = false }: Props) {
  const mediaKnown = Array.isArray(media);
  const asset = [...(media || [])].filter(item => item.asset_id).sort((a, b) =>
    String(b.created_at || "").localeCompare(String(a.created_at || "")) || a.asset_id.localeCompare(b.asset_id)
  )[0];
  const src = asset ? urlForAsset(asset.asset_id) : "";
  const [failedSource, setFailedSource] = useState("");
  const failed = !!src && failedSource === src;
  const status = failed ? "图片加载失败 · 类别示意" : mediaKnown ? "尚无关联图片 · 类别示意" : "图片资料待读取 · 类别示意";
  return (
    <span className="wardrobe-item-art relative flex size-full min-h-0 items-center justify-center" data-media-state={failed ? "failed" : src ? "linked" : mediaKnown ? "unlinked" : "unknown"}>
      {src && !failed ? (
        <img className="size-full min-h-0 object-contain" src={src} alt={name + "的关联图片"} onError={() => setFailedSource(src)} />
      ) : (
        <>
          <GarmentArt type={type} name={name} />
          {showStatus && <span className="absolute inset-x-2 bottom-2 rounded bg-background/90 px-2 py-1 text-center text-xs text-muted-foreground">{status}{failed && allowRetry && <Button type="button" size="sm" variant="outline" className="ml-2" onClick={() => setFailedSource("")}>重新加载图片</Button>}</span>}
        </>
      )}
    </span>
  );
}

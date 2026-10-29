import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Disclosure } from "../controls";
import useWorkspaceRead from "./useWorkspaceRead";
import { FeedReader, FeedMore } from "./FeedParts";
export default function FeedMediaPicker({ model: m, ids, onChange }) {
  const [open, setOpen] = useState(false),
    [names, setNames] = useState({}),
    [error, setError] = useState(""),
    epoch = useRef(0);
  const source = useWorkspaceRead({
    identity: ["feed-picker"],
    characterScoped: false,
    enabled: open,
    read: (cursor) => m.w.services.mediaClient.list({ cursor, hidden: false }),
  });
  useEffect(() => {
    const token = ++epoch.current;
    for (const id of ids)
      void m.w.services.mediaClient
        .asset(id)
        .then((row) => {
          if (m.live() && token === epoch.current && row.id === id)
            setNames((old) => ({
              ...old,
              [id]: row.title || row.source_filename || "已选附件",
            }));
        })
        .catch(() => {});
    return () => {
      epoch.current++;
    };
  }, [ids.join("|")]);
  return (
    <div className="companion-form-stack">
      <div className="lp-action-row">
        {ids.map((id) => (
          <Button
            key={id}
            type="button"
            variant="outline"
            onClick={() => onChange(ids.filter((x) => x !== id))}
          >
            移除 {names[id] || "已选附件（名称待读取）"}
          </Button>
        ))}
      </div>
      <Disclosure
        title={"从我的素材库选择附件 · " + ids.length + "/8"}
        open={open}
        onOpenChange={setOpen}
      >
        <FeedReader source={source} label="可选素材" />
        {source.ready && !source.items.length && (
          <p>素材库为空，可以先上传或生成素材。</p>
        )}
        {source.items.map((a) => (
          <div className="feed-picker-row" key={a.id}>
            <span>
              {a.title || a.source_filename || "未命名素材"} ·{" "}
              {{ image: "图片", audio: "音频", video: "视频" }[a.kind] ||
                "素材"}{" "}
              · {a.readable ? "可读取" : "暂不可读取"}
            </span>
            <Button
              type="button"
              variant="outline"
              disabled={!a.readable || ids.includes(a.id)}
              onClick={() => {
                if (ids.length >= 8) {
                  setError("最多选择8项附件。");
                  return;
                }
                setError("");
                onChange([...ids, a.id]);
              }}
            >
              {ids.includes(a.id) ? "已选择" : "选择"}
            </Button>
          </div>
        ))}
        <FeedMore source={source} label="素材" />
      </Disclosure>
      {error && <p role="alert">{error}</p>}
      <Button asChild variant="outline"><a href="#/studio/media">上传或管理素材</a></Button>
      <p className="companion-caption">
        附件保留原来源，不证明画面中的事件实际发生。每条最多8项。
      </p>
    </div>
  );
}

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Disclosure, Textarea } from "../controls";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import FeedPost from "./FeedPost";
import { FeedReader, FeedMore } from "./FeedParts";
import {
  checkMoment,
  momentAuthor,
  momentCount,
  momentText,
} from "@/approved-ui/feed-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
function CommentComposer({ model: m, post, onSaved, onClose }) {
  const [action] = useState(() => ({
    identity: "comment:" + post.id,
    presentation: "inline",
    scope: ["feed-comment", post.id],
    storageKey: `fluctlight-studio:${m.owner}:moment:replica:${post.id}:comment`,
    title: "评论这个片刻",
    submitLabel: "发布评论",
    canSubmit: (d) => !!d?.text?.trim(),
    cancelLabel: "收起评论",
    initial: { text: "", revision: post.revision },
    validate: (d) => momentText(d.text),
    build: (d) => ({
      text: momentText(d.text),
      expectedRevision: d.revision,
      reason: "所有者以自己的身份评论动态",
    }),
    execute: async (body, key) => {
      m.assertCurrent();
      return checkMoment(
        await m.service.comment(post.id, { ...body, idempotencyKey: key }),
        post.id,
      );
    },
    readLatest: () => m.readMoment(post.id),
    adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
    onSaved: async (r, body) => {
      await onSaved(r, body);
    },
    fields: (d, edit) => (
      <Textarea
        aria-label="评论内容"
        value={d.text}
        placeholder="以自己的身份，说点什么…"
        rows={2}
        onChange={(e) => edit({ text: e.target.value })}
      />
    ),
  }));
  return (
    <div className="companion-comment-composer">
      <WorkspaceCommandDialog action={action} onClose={onClose} />
    </div>
  );
}
function NPCInteractions({ model: m, post }) {
  const interactions = useWorkspaceRead({
      identity: ["feed-npc", post.id],
      characterScoped: false,
      read: (cursor) => m.service.npcInteractions(post.id, cursor),
    }),
    source = useRef(null),
    [action, setAction] = useState(null);
  function retry(row, event) {
    m.assertCurrent();
    source.current = event.currentTarget;
    setAction({
      identity: "npc-retry:" + post.id + ":" + row.id,
      scope: ["feed-npc", post.id, row.id],
      storageKey: `fluctlight-studio:${m.owner}:moment:replica:${post.id}:npc:${row.id}`,
      title: "重新核对这次人物互动",
      description:
        "核对原结识来源、人物定义和运行世界；不会强制评论，也不会建立新关系。",
      initial: { revision: row.revision },
      submitLabel: "确认重新核对",
      build: (d) => ({
        expectedRevision: d.revision,
        reason: "所有者重新核对原人物动态互动",
      }),
      execute: async (body, key) => {
        m.assertCurrent();
        const r = await m.service.retryNPC(post.id, row.id, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.id !== row.id ||
          r.moment_id !== post.id ||
          !Number.isSafeInteger(r.revision)
        )
          throw Error("互动重试回执尚未完整确认。");
        return r;
      },
      readLatest: () => m.service.npcInteraction(post.id, row.id),
      adoptLatest: (d, fresh) => ({ ...d, revision: fresh.revision }),
      onSaved: async (r) => {
        const fresh = await m.service.npcInteraction(post.id, row.id);
        if (
          fresh.id !== r.id ||
          fresh.moment_id !== post.id ||
          fresh.revision < r.revision
        )
          throw Error("原互动的新版本尚未读回。");
        if (m.live()) await interactions.load();
      },
      fields: () => (
        <p className="companion-body-text">
          只重新评估这次原有互动，实际结果仍由角色判断。
        </p>
      ),
    });
  }
  return (
    <>
      <Disclosure title="世界人物互动与实际来源">
        <WorkspaceInfo>
          已有结识来源的人物可以独立判断是否回应。远程互动不表示同场，也不会新建关系。
        </WorkspaceInfo>
        <FeedReader source={interactions} label="人物互动" />
        {interactions.ready && !interactions.items.length && (
          <p>
            没有已受理的人物互动。全局用户动态或尚未结识的人物不会生成互动任务。
          </p>
        )}
        {interactions.items.map((row) => (
          <div className="feed-npc-row" key={row.id}>
            <strong>
              {{
                pending: "等待独立判断",
                completed: "实际互动已提交",
                skipped: "本次不回应",
                suppressed: "来源变化，未产生互动",
                failed: "执行失败",
              }[row.status] || "互动状态待读取"}
            </strong>
            <p>
              {row.response?.reason || row.error_code || "实际结果尚未提供。"}
            </p>
            {(row.tasks || []).map((t) => (
              <a
                key={t.task_id}
                href={"#/diag/task/" + encodeURIComponent(t.task_id)}
              >
                查看原任务
              </a>
            ))}
            {["failed", "suppressed"].includes(row.status) && (
              <Button variant="outline" onClick={(e) => retry(row, e)}>
                重试本次互动
              </Button>
            )}
            <Disclosure title="原结识、来源与互动记录">
              <pre className="replica-source-record">
                {JSON.stringify(row, null, 2)}
              </pre>
            </Disclosure>
          </div>
        ))}
        <FeedMore source={interactions} label="人物互动" />
      </Disclosure>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => source.current}
        />
      )}
    </>
  );
}
export default function FeedDetail({ model: m, id, onClose }) {
  const detail = useWorkspaceRead({
    identity: ["feed-detail", id],
    characterScoped: false,
    read: async () => ({ items: [await m.readMoment(id)], next_cursor: "" }),
  });
  const comments = useWorkspaceRead({
    identity: ["feed-comments", id],
    characterScoped: false,
    read: (cursor) => m.service.comments(id, cursor),
  });
  const [commentOpen, setCommentOpen] = useState(true),
    [commentEpoch, setCommentEpoch] = useState(0),
    [notice, setNotice] = useState(""),
    detailRef = useRef(detail),
    commentsRef = useRef(comments),
    commentButton = useRef(null);
  detailRef.current = detail;
  commentsRef.current = comments;
  const listed = m.feed.items.find((p) => p.id === id),
    stored = detail.items[0],
    post =
      listed && stored && listed.revision > stored.revision ? listed : stored;
  useEffect(() => {
    if (stored && m.live()) m.adoptPost(stored);
  }, [stored]);
  async function saved(r, body) {
    const [fresh, page] = await Promise.all([
      m.readMoment(id),
      m.service.comments(id, ""),
    ]);
    const matches = (c) =>
      c.moment_id === id &&
      c.author_actor_id === m.owner &&
      c.text === body.text;
    let found = page.items.some(matches),
      cursor = page.next_cursor;
    const seen = new Set([""]);
    while (!found && cursor) {
      m.assertCurrent();
      if (seen.has(cursor)) throw Error("评论分页位置重复，请重新核对原结果。");
      seen.add(cursor);
      const older = await m.service.comments(id, cursor);
      found = older.items.some(matches);
      cursor = older.next_cursor;
    }
    if (
      fresh.revision < r.revision ||
      fresh.comment_count < r.comment_count ||
      !found
    )
      throw Error("评论正文、作者或动态的新版本尚未完整读回。");
    if (m.live()) {
      detailRef.current.adopt({ items: [fresh], next_cursor: "" });
      commentsRef.current.adopt(page);
      m.adoptPost(fresh);
      setNotice("你的评论已保存并读回。");
    }
  }
  return (
    <WorkspaceDialog
      open
      title="一个生活片刻"
      description="动态正文、回应与实际来源"
      wide
      returnFocus={() => m.trigger.current}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <div className="replica-feed companion-form-stack">
        <FeedReader source={detail} label="动态详情" />
        <Button
          variant="outline"
          disabled={detail.busy}
          onClick={() => {
            void detail.load();
            void comments.load();
          }}
        >
          刷新动态与评论
        </Button>
        {post && (
          <>
            <FeedPost
              post={post}
              model={m}
              full
              onOpen={() => commentButton.current?.focus()}
              onAction={(kind, row, source, preset) =>
                m.interact(kind, row, source, preset, (fresh) =>
                  detailRef.current.adopt({ items: [fresh], next_cursor: "" }),
                )
              }
            />
            <section className="companion-comments">
              <h3>评论 · {momentCount(post.comment_count)}</h3>
              <FeedReader source={comments} label="评论" />
              {comments.ready && !comments.items.length && (
                <p>还没有回应，你可以留下第一句话。</p>
              )}
              {comments.items.map((c) => (
                <div key={c.id}>
                  <span className="companion-comment-avatar" aria-hidden="true">
                    {[...momentAuthor(c)].at(-1)}
                  </span>
                  <div>
                    <strong>
                      {momentAuthor(c)}
                      <small>
                        {c.created_at
                          ? userInstant(c.created_at)
                          : "时间未记录"}
                      </small>
                    </strong>
                    <p>{c.text}</p>
                    {c.source_interaction_id && (
                      <span className="companion-caption">
                        来自已保存的人物互动
                      </span>
                    )}
                  </div>
                </div>
              ))}
              <FeedMore source={comments} label="评论" />
            </section>
            {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
            {post.status === "hidden" ? (
              <WorkspaceInfo>
                这条动态已隐藏，恢复展示后可以继续评论。
              </WorkspaceInfo>
            ) : commentOpen ? (
              <CommentComposer
                key={id + ":" + commentEpoch}
                model={m}
                post={post}
                onSaved={saved}
                onClose={() => {
                  setCommentOpen(false);
                  setCommentEpoch((x) => x + 1);
                }}
              />
            ) : (
              <Button
                ref={commentButton}
                variant="outline"
                onClick={() => setCommentOpen(true)}
              >
                写评论或继续核对原提交
              </Button>
            )}
            <NPCInteractions key={id} model={m} post={post} />
            <div className="companion-source-note">
              作者：{momentAuthor(post)} ·{" "}
              {{
                user: "由你发布",
                fluctlight: "角色生活分享",
                npc: "世界人物分享",
              }[post.author_type] || "作者类型未记录"}
            </div>
          </>
        )}
      </div>
    </WorkspaceDialog>
  );
}

import { useState } from "react";
import { Heart, MessageCircle, ChevronRight, Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge, Disclosure } from "../controls";
import JsonDetails from './JsonDetails';
import MediaPreview from "./MediaPreview";
import CharacterAvatar from "./CharacterAvatar";
import useWorkspaceRead from "./useWorkspaceRead";
import { FeedReader, FeedTime, feedAuthorName, feedDisplayText } from "./FeedParts";
import { momentCount } from "@/approved-ui/feed-presentation.mjs";
import { momentReading, momentRecord } from "@/v3/moment-reading.mjs";
const visibilityLabels = { owner: "仅所有者可见", participants: "参与者可见" };

function PostMedia({ asset, model: m, full, onOpen }) {
  const [failed, setFailed] = useState(false);
  if (!asset?.id) return null;
  const readable = asset.readable === true || asset.read_state === "readable";
  if (!readable)
    return (
      <div className="feed-unavailable" role="status">
        附件暂不可读取，原来源仍保留。
        <a href={"#/studio/media/" + encodeURIComponent(asset.id)}>
          查看素材来源
        </a>
      </div>
    );
  const src = asset.url || m.w.services.mediaClient.url(asset.id);
  const label = [asset.title, asset.source_filename]
    .filter((value) => value !== asset.id)
    .map(feedDisplayText)
    .find(Boolean) || "动态附件";
  if (!full)
    return (
      <button
        className="companion-feed-image"
        aria-label="查看动态详情"
        onClick={onOpen}
      >
        {asset.kind === "image" && !failed ? (
          <img
            src={src}
            alt={label}
            className="feed-image"
            loading="lazy"
            onError={() => setFailed(true)}
          />
        ) : (
          <span className="feed-media-placeholder">
            {failed
              ? "附件预览读取失败"
              : asset.kind === "audio"
                ? "音频附件"
                : "视频附件"}{" "}
            · 打开详情查看
          </span>
        )}
      </button>
    );
  return (
    <div className="feed-full-media">
      {asset.kind === "image" ? (
        <MediaPreview src={src} alt={label} className="feed-image" />
      ) : failed ? (
        <p role="alert">附件播放失败，可以重新打开或查看来源。</p>
      ) : asset.kind === "audio" ? (
        <audio
          controls
          preload="metadata"
          src={src}
          onError={() => setFailed(true)}
        />
      ) : asset.kind === "video" ? (
        <video
          controls
          preload="metadata"
          src={src}
          onError={() => setFailed(true)}
        />
      ) : (
        <p>此附件类型请从来源查看。</p>
      )}
      <a href={"#/studio/media/" + encodeURIComponent(asset.id)}>
        查看媒体来源
      </a>
    </div>
  );
}
function CommentPreview({ post, model: m }) {
  const source = useWorkspaceRead({
    identity: ["feed-preview-comments", post.id, post.revision],
    characterScoped: false,
    enabled: post.comment_count > 0,
    read: (cursor) => m.service.comments(post.id, cursor),
  });
  return (
    <>
      <FeedReader source={source} label="评论预览" />
      {source.items.length > 0 && (
        <div className="companion-comment-preview">
          {source.items.slice(0, 2).map((c) => {
            const author = feedAuthorName(c);
            return <p key={c.id}>{author && <b>{author}：</b>}{c.text}</p>;
          })}
        </div>
      )}
    </>
  );
}
export default function FeedPost({
  post,
  model: m,
  full = false,
  onOpen,
  onAction,
}) {
  const reading = momentReading(post),
    author = feedAuthorName(post),
    hidden = post.status === "hidden",
    character = m.w.state.characters.find(c=>c.id===post.author_actor_id);
  return (
    <article
      className={"companion-feed-card" + (full ? " is-full" : "")}
      data-testid={"feed-post-" + post.id}
    >
      <header>
        <CharacterAvatar className="feed-author-avatar" name={author} src={character?._directory?.avatar} color={character?.color}/>
        <div>
          {author && <strong>{author}</strong>}
          <span><FeedTime value={post.created_at} /></span>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label={hidden ? "恢复展示动态" : "隐藏这条动态"}
          onClick={(event) =>
            onAction(hidden ? "restore" : "hide", post, event.currentTarget)
          }
        >
          {hidden ? <Eye size={16} /> : <EyeOff size={16} />}
        </Button>
      </header>
      {hidden && (
        <Badge variant="outline" className="companion-hidden-badge">
          已隐藏
        </Badge>
      )}
      <p className="companion-post-text">{reading.text}</p>
      {reading.corrected && (
        <Disclosure title="阅读版略去历史异常字符 · 查看原文">
          <p className="companion-post-text">{reading.original}</p>
        </Disclosure>
      )}
      <div className="feed-media-grid">
        {(post.media || []).map((asset) => (
          <PostMedia
            key={asset.id}
            asset={asset}
            model={m}
            full={full}
            onOpen={(event) => onOpen?.(post.id, event.currentTarget)}
          />
        ))}
      </div>
      <footer>
        <Button
          variant="ghost"
          className={post.viewer_reaction === "like" ? "liked" : ""}
          aria-label={
            post.viewer_reaction === "like" ? "取消点赞" : "赞这条动态"
          }
          aria-pressed={post.viewer_reaction === "like"}
          disabled={hidden || !m.momentEnabled}
          onClick={(event) =>
            onAction(
              "reaction",
              post,
              event.currentTarget,
              post.viewer_reaction === "like" ? "none" : "like",
            )
          }
        >
          <Heart
            size={17}
            fill={post.viewer_reaction === "like" ? "currentColor" : "none"}
          />
          {momentCount(post.like_count)}
        </Button>
        {!full&&<Button
          variant="ghost"
          onClick={(event) => onOpen?.(post.id, event.currentTarget)}
        >
          <MessageCircle size={17} />
          {post.comment_count ? momentCount(post.comment_count) : "评论"}
        </Button>}
        {!full && (
          <Button
            variant="ghost"
            className="companion-feed-detail-link"
            onClick={(event) => onOpen?.(post.id, event.currentTarget)}
          >
            查看详情
            <ChevronRight size={13} />
          </Button>
        )}
      </footer>
      {full ? (
        <Disclosure title="更多回应与实际来源">
          <div className="lp-action-row">
            {[
              ["care", "关心"],
              ["celebrate", "庆祝"],
            ].map(([kind, label]) => (
              <Button
                key={kind}
                variant="outline"
                disabled={hidden || !m.momentEnabled}
                aria-pressed={post.viewer_reaction === kind}
                onClick={(event) =>
                  onAction(
                    "reaction",
                    post,
                    event.currentTarget,
                    post.viewer_reaction === kind ? "none" : kind,
                  )
                }
              >
                {post.viewer_reaction === kind ? "取消" + label : label}
              </Button>
            ))}
          </div>
          {Number.isSafeInteger(post.reaction_count) && post.reaction_count >= 0 && (
            <p>全部回应 {momentCount(post.reaction_count)}</p>
          )}
          {post.viewer_reaction === "care" && <p>你已关心</p>}
          {post.viewer_reaction === "celebrate" && <p>你已庆祝</p>}
          {post.viewer_reaction === "like" && <p>你已点赞</p>}
          {post.viewer_reaction === null && <p>尚无你的回应</p>}
          {post.request_id && (
            <a href={"#/diag/request/" + encodeURIComponent(post.request_id)}>
              原发布建议与处理结果
            </a>
          )}
          {Object.hasOwn(visibilityLabels, post.visibility) && <p>可见范围：{visibilityLabels[post.visibility]}</p>}
          <JsonDetails value={momentRecord(post)} title="原始动态记录（诊断）" downloadOnly />
        </Disclosure>
      ) : (
        <CommentPreview post={post} model={m} />
      )}
    </article>
  );
}

import {
  Leaf,
  Plus,
  Search,
  Sparkles,
  RefreshCw,
  ChevronRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ChoiceSelect, FieldInput, Surface, Disclosure } from "../controls";
import {EmptyState} from "../shared/Presentation";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceCommandDialog from "../shared/WorkspaceCommandDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import RequestStatus from "../shared/RequestStatus";
import FeedPost from "../shared/FeedPost";
import FeedDetail from "../shared/FeedDetail";
import { FeedReader, FeedMore } from "../shared/FeedParts";
import {
  publishedMomentIds,
  feedHref,
} from "@/approved-ui/feed-presentation.mjs";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { requestDecisionText } from "@/v3/proto/components/request-failure.js";
import { userInstant } from "@/v3/presentation-time.mjs";
export default function FeedPageView({ model: m }) {
  const rows = m.feed.items.filter(
    (row) => m.filters.author !== "me" || row.author_type === "user",
  );
  return (
    <div className="replica-feed companion-page">
      <WorkspacePageHeader
        eyebrow="WORLD / SMALL MOMENTS"
        title="日子里的小小回响。"
        description="一些想分享的事，和那些愿意回应的人。"
        actions={
          <>
            <Button
              variant="ghost"
              size="icon"
              aria-label="刷新动态"
              disabled={m.feed.busy}
              onClick={() => {
                void m.feed.load();
                void m.suggestions.load();
              }}
            >
              <RefreshCw size={15} />
            </Button>
            <Button
              variant="outline"
              onClick={(e) => m.compose("suggest", e.currentTarget)}
            >
              <Sparkles size={15} />
              邀请分享近况
            </Button>
            <Button onClick={(e) => m.compose("publish", e.currentTarget)}>
              <Plus size={15} />
              我来发动态
            </Button>
          </>
        }
      />
      <form
        className="companion-toolbar feed-toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          m.navigateFilters({ q: m.search });
        }}
      >
        <ChoiceSelect
          aria-label="动态角色范围"
          value={m.filters.author === "me" ? "me" : m.filters.fid}
          onChange={(e) =>
            m.navigateFilters({
              fid: e.target.value === "me" ? "" : e.target.value,
              author: e.target.value === "me" ? "me" : "",
            })
          }
        >
          <option value="">所有人的动态</option>
          <option value="me">我发布的</option>
          {m.w.state.characters.map((c) => (
            <option value={c.id} key={c.id}>
              {c.name}
            </option>
          ))}
        </ChoiceSelect>
        <div className="feed-search">
          <Search size={15} />
          <FieldInput
            type="search"
            aria-label="寻找一个片刻"
            placeholder="寻找一个片刻…"
            value={m.search}
            onChange={(e) => m.setSearch(e.target.value)}
          />
          <Button
            variant="ghost"
            type="submit"
            size="icon"
            aria-label="搜索动态"
          >
            <ChevronRight size={14} />
          </Button>
        </div>
        <ChoiceSelect
          aria-label="动态展示状态"
          value={m.filters.status}
          onChange={(e) => m.navigateFilters({ status: e.target.value })}
        >
          <option value="visible">可见动态</option>
          <option value="hidden">已隐藏</option>
          <option value="all">全部动态</option>
        </ChoiceSelect>
      </form>
      {m.filterError && <p role="alert">{m.filterError}</p>}
      {!m.action &&
        m.composerRecovery.map((mode) => (
          <WorkspaceInfo key={mode}>
            上次{mode === "publish" ? "动态发布" : "分享建议"}
            的输入或回执仍保留。
            <Button
              variant="outline"
              onClick={(event) => m.compose(mode, event.currentTarget)}
            >
              继续核对{mode === "publish" ? "动态发布" : "分享建议"}
            </Button>
          </WorkspaceInfo>
        ))}
      {m.feedback && <WorkspaceInfo>{m.feedback}</WorkspaceInfo>}
      {m.filters.author === "me" && (
        <p className="companion-caption">
          “我发布的”筛选已读动态；可继续加载更早记录。
        </p>
      )}
      {m.filters.fid && (
        <p className="companion-caption">
          当前为这位角色的动态范围，包含其中不同作者的分享。
        </p>
      )}
      <div className="companion-feed-layout">
        <div className="companion-feed-list">
          <FeedReader source={m.feed} label="动态" />
          {rows.map((row) => (
            <FeedPost
              key={row.id}
              post={row}
              model={m}
              onOpen={m.openPost}
              onAction={m.interact}
            />
          ))}
          {m.feed.ready && !m.feed.busy && !m.feed.error && !rows.length && (
            <EmptyState variant="page">
                <h3>还没有这样的片刻</h3>
                <p>
                  {m.feed.cursor
                    ? "已读记录中没有符合筛选的动态，可以继续加载更早记录。"
                    : "试试其他关键词，或者先发布一条属于你的动态。"}
                </p>
                <Button
                  variant="outline"
                  onClick={() =>
                    m.navigateFilters({
                      fid: "",
                      author: "",
                      status: "visible",
                      q: "",
                    })
                  }
                >
                  清除筛选
                </Button>
            </EmptyState>
          )}
          <FeedMore source={m.feed} label="动态" />
        </div>
        <aside className="companion-stack">
          <Surface className="companion-feed-note">
            <Leaf size={22} />
            <h3>
              普通的一天，
              <br />
              也有值得留下的事。
            </h3>
            <p>
              不必想好一个特别的故事。分享一阵雨、一束光，或刚读过的一句话就很好。
            </p>
          </Surface>
          <Surface className="feed-suggestions">
            <h2>你们的分享建议</h2>
            <FeedReader source={m.suggestions} label="分享建议" />
            {m.suggestions.items.map((r) => (
              <button
                className="companion-suggestion-row"
                key={r.id}
                onClick={() =>
                  m.setSuggestion({
                    id: r.id,
                    fid: r.fluctlight_id,
                    initial: r,
                  })
                }
              >
                <p>{r.payload?.text || "原建议正文未提供"}</p>
                <span>
                  {r.created_at ? userInstant(r.created_at) : "时间未记录"} ·{" "}
                  {stateLabel(r.status)}
                </span>
                <ChevronRight size={13} />
              </button>
            ))}
            {m.suggestions.ready && !m.suggestions.items.length && (
              <p className="companion-body-text">
                你可以提出建议，由角色判断想不想分享。实际发布后可以在这里查看结果。
              </p>
            )}
            <FeedMore source={m.suggestions} label="分享建议" />
            {m.hasLastSuggestion() && (
              <Button variant="outline" onClick={m.resumeSuggestion}>
                继续查看上次分享建议
              </Button>
            )}
          </Surface>
        </aside>
      </div>
      <Disclosure
        className="feed-mobile-suggestions"
        title="你们的分享建议与原请求"
      >
        <FeedReader source={m.suggestions} label="分享建议" />
        {m.suggestions.items.map((r) => (
          <Button
            key={r.id}
            variant="outline"
            onClick={() =>
              m.setSuggestion({ id: r.id, fid: r.fluctlight_id, initial: r })
            }
          >
            {r.payload?.text || "分享建议"} · {stateLabel(r.status)}
          </Button>
        ))}
        {m.suggestions.ready && !m.suggestions.items.length && (
          <p>还没有分享建议。</p>
        )}
        <FeedMore source={m.suggestions} label="分享建议" />
        {m.hasLastSuggestion() && (
          <Button variant="outline" onClick={m.resumeSuggestion}>
            继续查看上次分享建议
          </Button>
        )}
      </Disclosure>
      {m.selected && (
        <FeedDetail
          key={m.selected}
          model={m}
          id={m.selected}
          onClose={() => {
            m.setSelected(null);
            if (m.initialPostId) m.w.navigatePath(m.returnHref.slice(1));
          }}
        />
      )}
      {m.action && (
        <WorkspaceCommandDialog
          key={m.action.identity}
          action={m.action}
          onClose={m.closeAction}
          returnFocus={() => m.commandTrigger.current}
        />
      )}
      {m.suggestion && !m.action && (
        <WorkspaceDialog
          open
          title="这次分享建议"
          description="建议、角色判断和实际发布分别保留"
          onOpenChange={(v) => {
            if (!v) m.setSuggestion(null);
          }}
        >
          <Suggestion model={m} />
        </WorkspaceDialog>
      )}
    </div>
  );
}
function Suggestion({ model: m }) {
  const r = m.suggestion.initial,
    published = publishedMomentIds(r);
  return (
    <div className="replica-feed companion-form-stack">
      {r?.payload?.text && (
        <p className="companion-post-text">{r.payload.text}</p>
      )}
      {r && <p>{requestDecisionText(r)}</p>}
      <RequestStatus
        key={m.suggestion.id}
        ownerScoped
        fid={m.suggestion.fid}
        requestId={m.suggestion.id}
        initial={r}
        onTerminal={async (latest) => {
          if (m.live()) {
            m.setSuggestion((old) =>
              old?.id === latest.id ? { ...old, initial: latest } : old,
            );
            await m.suggestions.load();
            await m.feed.load();
          }
        }}
      />
      {published.map((id) => (
        <Button
          key={id}
          variant="outline"
          onClick={() => {
            m.setSuggestion(null);
            m.openPost(id, null);
          }}
        >
          查看角色实际发布
        </Button>
      ))}
      {r?.status === "failed" && (
        <Button
          variant="outline"
          onClick={(e) => {
            m.setSuggestion(null);
            m.compose("suggest", e.currentTarget, {
              requestId: r.id,
              fid: r.fluctlight_id,
              text: r.payload?.text || "",
              mediaIds: r.payload?.media_ids || [],
            });
          }}
        >
          重新和她商量
        </Button>
      )}
    </div>
  );
}

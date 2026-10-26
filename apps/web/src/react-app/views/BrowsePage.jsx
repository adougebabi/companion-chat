import { DrawerFrame } from "@/react-app/controls";
import { readValue } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  Textarea,
  DialogFrame,
  Badge,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  listValues,
  Fragment,
} from "@/react-app/render-helpers";
import { LeafIcon } from "lucide-react";
export function BrowsePageView({ view: view }) {
  return view.props.page === "动态" ? (
    <Fragment key={0}>
      <p className="muted">{"生活的切片，留一点位置给彼此的回应。"}</p>
      <Fragment>
        {listValues(readValue(view.character).posts).map((_, ee) => {
          var he;
          return (
            <Surface as="article" key={ee} className="card post compact">
              <div className="row">
                <span className="avatar small">
                  {displayText(_.authorInitial)}
                </span>
                <b>{displayText(_.authorName)}</b>
                <small>{displayText(_.time)}</small>
              </div>
              <p>{displayText(_.text)}</p>
              <div className="post-art">
                <LeafIcon size={36} />
                <span>{"把日子过成一首小诗"}</span>
              </div>
              <div className="row">
                <ActionButton
                  className="text-button"
                  onClick={() => view.reactPost(_)}
                >
                  {displayText(_.liked ? "♥" : "♡") +
                    " " +
                    displayText(_.likes) +
                    " 喜欢"}
                </ActionButton>
                <ActionButton
                  data-testid={"comment-toggle-" + ee}
                  className="text-button"
                  onClick={(j) =>
                    (view.commentsOpen.value[ee] = !view.commentsOpen.value[ee])
                  }
                >
                  {"评论 " +
                    displayText(_.comments.length) +
                    " " +
                    displayText(view.commentsOpen.value[ee] ? "收起" : "展开")}
                </ActionButton>
              </div>
              {view.commentsOpen.value[ee] ? (
                <div key={0} className="comments">
                  <Fragment>
                    {listValues(_.comments).map((j, T) => (
                      <p key={T}>
                        <span className="avatar small">
                          {displayText(j.name.slice(0, 1))}
                        </span>
                        <span>
                          <b>{displayText(j.name)}</b>
                          {""}
                          <small>{displayText(j.time)}</small>
                          <br />
                          {displayText(j.text)}
                        </span>
                      </p>
                    ))}
                  </Fragment>
                  <form
                    className="input-row"
                    onSubmit={eventModifiers(
                      (j) => view.comment(ee),
                      ["prevent"],
                    )}
                  >
                    <Textarea
                      placeholder="留下一句回应…"
                      aria-label="评论内容"
                      rows="2"
                      data-testid={"comment-input-" + ee}
                      onKeyDown={(j) => view.onCommentKeyDown(j, ee)}
                      value={view.commentDrafts.value[ee]}
                      onChange={(event) => {
                        ((j) => (view.commentDrafts.value[ee] = j))(
                          event.target.value,
                        );
                      }}
                    />
                    <ActionButton
                      className="button"
                      disabled={
                        readValue(view.busy) ||
                        !(
                          (he = view.commentDrafts.value[ee]) != null &&
                          he.trim()
                        )
                      }
                    >
                      {displayText(
                        readValue(view.busy) && view.commentTarget.value === ee
                          ? "处理中…"
                          : "发表评论",
                      )}
                    </ActionButton>
                  </form>
                </div>
              ) : null}
            </Surface>
          );
        })}
      </Fragment>
      <view.ActionFeedback labels={["动态"]} />
    </Fragment>
  ) : view.props.page === "人物关系" ? (
    <Fragment key={1}>
      <view.FilterChips
        search={view.search.value}
        onUpdateSearch={(_) => (view.search.value = _)}
        filters={view.filters.value}
        onUpdateFilters={(_) => (view.filters.value = _)}
        groups={view.peopleFilters.value}
        testId="people-filters"
        placeholder="搜索姓名 / 身份"
      />
      <small className="muted">
        {displayText(view.people.value.length) + " 位相识"}
      </small>
      <div className="people-grid">
        <Fragment>
          {listValues(view.people.value).map((_) => (
            <ActionButton
              key={_.id}
              className="card person-card"
              onClick={(ee) => (view.selected.value = _.id)}
            >
              <span className="avatar">{displayText(_.name.slice(0, 1))}</span>
              <h3>{displayText(_.name)}</h3>
              <p>{displayText(_.identity) + " · " + displayText(_.relation)}</p>
              <small>
                {displayText(_.location) + " · " + displayText(_.tag)}
              </small>
            </ActionButton>
          ))}
        </Fragment>
      </div>
      {view.people.value.length ? null : (
        <Surface as="div" key={0} className="card empty-state">
          <p>{"暂无匹配人物"}</p>
          <ActionButton className="button" onClick={view.clear}>
            {"清除筛选"}
          </ActionButton>
        </Surface>
      )}
      {view.person.value ? (
        <DrawerFrame
          key={1}
          data-testid="person-drawer-overlay"
          className="drawer-backdrop"
          onClose={(_) => (view.selected.value = null)}
        >
          <aside
            data-testid="person-drawer"
            className="drawer-form"
            aria-label={view.person.value.name}
          >
            <ActionButton
              className="button"
              onClick={(_) => (view.selected.value = null)}
            >
              {"关闭"}
            </ActionButton>
            <h2>{displayText(view.person.value.name)}</h2>
            <p>
              {displayText(view.person.value.identity) +
                " · " +
                displayText(view.person.value.type) +
                " · " +
                displayText(view.person.value.status)}
            </p>
            <p>{displayText(view.person.value.bio)}</p>
            <p>
              {"关系：" + displayText(view.person.value.relation)}
              <br />
              {"位置：" + displayText(view.person.value.location)}
              <br />
              {"日程：" + displayText(view.person.value.schedule)}
              <br />
              {"标签：" + displayText(view.person.value.tag)}
            </p>
          </aside>
        </DrawerFrame>
      ) : null}
    </Fragment>
  ) : view.props.page === "媒体" ? (
    <Fragment key={2}>
      <view.FilterChips
        search={view.search.value}
        onUpdateSearch={(_) => (view.search.value = _)}
        filters={view.filters.value}
        onUpdateFilters={(_) => (view.filters.value = _)}
        groups={view.mediaFilters.value}
        testId="media-filters"
        placeholder="搜索媒体 / 角色"
      />
      <small className="muted">
        {displayText(view.filteredMedia.value.length) + " 项"}
      </small>
      <div className="media-grid">
        <Fragment>
          {listValues(view.filteredMedia.value).map((_, rowIndex) => (
            <Surface
              as="article"
              className="card media-tile"
              key={typeof _ === "object" ? (_?.id ?? rowIndex) : _}
            >
              <ActionButton
                className="media-open"
                onClick={(ee) => (view.selected.value = _.id)}
              >
                <div
                  className={cn([
                    "media-preview",
                    "media-art-" + (_.artIndex % 6),
                    "media-type-" + _.type,
                    _.type === "图片" ? "photo" : "",
                  ])}
                >
                  {view.mediaContent(_, false)}
                </div>
                <b>{displayText(_.title)}</b>
                <small>
                  {displayText(_.character) +
                    " · " +
                    displayText(_.type) +
                    " · " +
                    displayText(_.time)}
                </small>
              </ActionButton>
              {_.type === "语音" ? (
                <div key={0} className="audio-control">
                  <ActionButton
                    className="button"
                    onClick={(ee) => view.playMedia(_.id)}
                  >
                    {displayText(view.playing.value === _.id ? "暂停" : "播放")}
                  </ActionButton>
                  <progress value={view.progress.value[_.id] || 0} max="100" />
                  <small>{displayText(view.mediaTime(_.id))}</small>
                </div>
              ) : null}
            </Surface>
          ))}
        </Fragment>
      </div>
      {view.selected.value !== null ? (
        <DialogFrame
          key={0}
          className="modal-backdrop"
          onClose={(_) => (view.selected.value = null)}
        >
          <Surface
            as="section"
            data-testid="media-preview-drawer"
            className="card media-modal"
            aria-label="媒体预览"
          >
            <Fragment>
              {listValues(
                readValue(view.allMedia).filter(
                  (_) => _.id === view.selected.value,
                ),
              ).map((_, rowIndex) => (
                <Fragment key={typeof _ === "object" ? (_?.id ?? rowIndex) : _}>
                  <div className="row">
                    <h2>{displayText(_.title)}</h2>
                    <span className="spacer" />
                    <ActionButton
                      className="button"
                      onClick={(ee) => (view.selected.value = null)}
                    >
                      {"关闭预览"}
                    </ActionButton>
                  </div>
                  <div
                    className={cn([
                      "media-preview",
                      "large-media",
                      "media-art-" + (_.artIndex % 6),
                      "media-type-" + _.type,
                    ])}
                  >
                    {view.mediaContent(_, true)}
                  </div>
                  <p>
                    {displayText(_.character) +
                      " · " +
                      displayText(_.time) +
                      " · " +
                      displayText(_.tag)}
                  </p>
                </Fragment>
              ))}
            </Fragment>
          </Surface>
        </DialogFrame>
      ) : null}
      {view.filteredMedia.value.length ? null : (
        <Surface as="div" key={1} className="card empty-state">
          <p>{"暂无匹配媒体。"}</p>
          <ActionButton
            className="button"
            data-testid="media-empty-clear"
            onClick={view.clear}
          >
            {"清除筛选"}
          </ActionButton>
        </Surface>
      )}
    </Fragment>
  ) : view.props.page === "角色库" ? (
    <Fragment key={3}>
      <ActionButton className="button" onClick={view.createCharacter}>
        {"认识新的人"}
      </ActionButton>
      <view.FilterChips
        search={view.search.value}
        onUpdateSearch={(_) => (view.search.value = _)}
        filters={view.filters.value}
        onUpdateFilters={(_) => (view.filters.value = _)}
        groups={view.libraryFilters.value}
        testId="library-filters"
        placeholder="搜索角色 / 职业"
      />
      <Fragment>
        {listValues(view.library.value).map((_) => (
          <Surface as="article" key={_.id} className="card compact">
            <div className="row">
              <span
                className="avatar"
                style={{
                  background: _.color,
                }}
              >
                {displayText(_.initial)}
              </span>
              <div>
                <b>{displayText(_.name) + " · " + displayText(_.job)}</b>
                <p>{displayText(_.subtitle)}</p>
                <Fragment>
                  {listValues(_.tags).map((ee, rowIndex) => (
                    <Badge
                      className="pill"
                      key={typeof ee === "object" ? (ee?.id ?? rowIndex) : ee}
                    >
                      {displayText(ee)}
                    </Badge>
                  ))}
                </Fragment>
              </div>
              <span className="spacer" />
              <ActionButton
                className="text-button"
                onClick={(ee) =>
                  (view.selected.value =
                    view.selected.value === _.id ? null : _.id)
                }
              >
                {displayText(
                  view.selected.value === _.id ? "收起详情" : "角色详情",
                )}
              </ActionButton>
              {readValue(view.selectedId) === _.id ? (
                <Badge key={0} className="pill current-badge">
                  {"当前"}
                </Badge>
              ) : (
                <ActionButton
                  key={1}
                  className="button primary"
                  onClick={(ee) => (view.selectedId.value = _.id)}
                >
                  {"切换角色"}
                </ActionButton>
              )}
            </div>
            {view.selected.value === _.id ? (
              <div key={0} className="inline-profile">
                <h2>{displayText(_.name)}</h2>
                <p>{displayText(_.bio)}</p>
                <p>
                  {"职业：" + displayText(_.job)}
                  <br />
                  {"兴趣：" + displayText(_.hobby)}
                  <br />
                  {"心情：" + displayText(_.mood)}
                  <br />
                  {"位置：" + displayText(_.location)}
                  <br />
                  {"世界：" + displayText(_.world)}
                </p>
              </div>
            ) : null}
          </Surface>
        ))}
      </Fragment>
      {view.library.value.length ? null : <p key={0}>{"暂无匹配角色。"}</p>}
    </Fragment>
  ) : view.props.page === "创作中心" ? (
    <Surface as="div" key={4} className="card placeholder">
      <h2>{"创作资源"}</h2>
      <p>
        {
          "在创作工作台编辑世界、具体地点、物品和人物定义。保存、发布、应用分别确认；已发布资源可随时继续编辑。"
        }
      </p>
      <div className="row">
        {[
          ["world", "新建世界"],
          ["place", "新建地点"],
          ["item", "新建物品"],
        ].map(([kind, label], rowIndex) => (
          <ActionButton
            className="button"
            onClick={() => view.createStudioResource(kind)}
            key={rowIndex}
          >
            {label}
          </ActionButton>
        ))}
      </div>
      {readValue(view.resources).map((resource) => (
        <article className="card compact" key={resource.id}>
          <h3>{resource.name}</h3>
          <p className="muted">
            {view.studioKind(resource.kind) +
              " · " +
              (resource.published_version ? "已有发布版本" : "草稿")}
          </p>
          <ActionButton
            className="button"
            onClick={() => view.openStudioResource(resource.id)}
          >
            {"编辑资源"}
          </ActionButton>
        </article>
      ))}
      {readValue(view.resources).length ? null : (
        <p className="muted">{"暂无创作资源，可以先新建地点或物品。"}</p>
      )}
    </Surface>
  ) : view.props.page === "通知与待处理" ? (
    <Fragment key={5}>
      <h2>{"未读通知"}</h2>
      {readValue(view.characters).length ? null : (
        <p className="muted">{"暂时没有未读通知。"}</p>
      )}
      {readValue(view.characters).map((group) => (
        <view.NotificationGroup
          key={group.id}
          group={group}
          onOpen={view.openNotificationOrCharacter}
        />
      ))}
      <h2>
        {"异常检查 "}
        <small>{displayText(view.errors.value.length) + " 项"}</small>
      </h2>
      <view.NotificationIssues
        groups={view.issueGroups.value}
        onOpen={view.openIssue}
      />
      {view.errors.value.length ? null : (
        <p key={0} className="muted">
          {"目前没有异常。"}
        </p>
      )}
    </Fragment>
  ) : null;
}

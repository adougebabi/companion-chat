import { isComposingKey } from "@/react-app/render-helpers";
import { useController } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { Fragment } from "@/react-app/render-helpers";
import { readNotificationPreviews } from "@/approved-ui/notification-previews.mjs";
import {
  pendingIssues,
  notificationGroups,
  notificationIssueGroups,
  notificationPath,
} from "@/approved-ui/presentation.mjs";
import NotificationGroup from "@/react-app/shared/NotificationGroup";
import NotificationIssues from "@/react-app/shared/NotificationIssues";
import { BrowsePageView } from "@/react-app/views/BrowsePage";
import { FilterChips, ActionFeedback } from "@/react-app/components";
function createBrowsePageController(props, context, lifecycle) {
  const w = lifecycle.workspace,
    character = w.current,
    playing = lifecycle.cell(null),
    progress = lifecycle.cell({}),
    search = lifecycle.cell(""),
    filters = lifecycle.cell({
      status: "",
      tag: "",
      scene: "",
      type: "",
      who: props.page === "媒体" ? character.value.name : "",
    }),
    selected = lifecycle.cell(null),
    commentDrafts = lifecycle.cell({}),
    commentsOpen = lifecycle.cell({}),
    commentTarget = lifecycle.cell(null);
  let playback = null;
  const durations = lifecycle.cell({}),
    mediaErrors = lifecycle.cell({}),
    scope = w.currentScope();
  let active = true;
  const live = () => active && w.currentScope() === scope;
  const noticePreviews = lifecycle.cell({});
  let noticeRead = 0;
  lifecycle.observe(
    () =>
      props.page === "通知与待处理"
        ? w.state.notifications.map((n) => n.id + "|" + n.link).join("\n")
        : "",
    async () => {
      const token = ++noticeRead;
      if (props.page !== "通知与待处理") return;
      const previews = await readNotificationPreviews(
        w.client,
        w.state.notifications,
        () => live() && props.page === "通知与待处理",
      );
      if (live() && token === noticeRead) noticePreviews.value = previews;
    },
    {
      immediate: true,
    },
  );
  const busy = selector(() =>
    w.state.operations.some(
      (r) =>
        r.fid === character.value.id &&
        r.label === "动态" &&
        r.status === "pending",
    ),
  );
  const matches = (text) =>
    String(text).toLowerCase().includes(search.value.trim().toLowerCase());
  const people = selector(() =>
    character.value.people.filter(
      (p) =>
        matches(p.name + " " + p.identity) &&
        (!filters.value.status || filters.value.status === p.status) &&
        (!filters.value.tag || filters.value.tag === p.tag) &&
        (!filters.value.scene || filters.value.scene === p.location),
    ),
  );
  const person = selector(() =>
    character.value.people.find((p) => p.id === selected.value),
  );
  const characters = selector(() =>
    props.page === "通知与待处理"
      ? notificationGroups(w.state.notifications, w.state.characters).map(
          (group) => ({
            ...group,
            notices: group.notices.map((notice) => ({
              ...notice,
              preview: noticePreviews.value[notice.id] || "",
            })),
          }),
        )
      : w.state.characters,
  );
  const library = selector(() =>
    characters.value.filter(
      (c) =>
        matches(c.name + " " + c.job + " " + c.bio) &&
        (!filters.value.tag || c.tags.includes(filters.value.tag)),
    ),
  );
  const media = selector(() =>
    w.state.media.filter(
      (a) =>
        matches(a.title + " " + a.character) &&
        (!filters.value.type || a.type === filters.value.type) &&
        (!filters.value.who || a.character === filters.value.who) &&
        (!filters.value.tag || a.tag === filters.value.tag),
    ),
  );
  const clear = () => {
    search.value = "";
    filters.value = {
      status: "",
      tag: "",
      scene: "",
      type: "",
      who: "",
    };
  };
  async function comment(index) {
    const post = character.value.posts[index],
      text = commentDrafts.value[index]?.trim(),
      fid = character.value.id;
    if (!post || !text || busy.value) return;
    commentTarget.value = index;
    const record = w.command(
      "动态",
      "comment:" + post.id,
      {
        text,
        expectedRevision: post._source.revision,
        reason: "以自己的交流身份评论动态",
      },
      (body, key) =>
        w.services.momentsClient.comment(post.id, {
          ...body,
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadBrowse("动态"),
        success: "评论已保存",
      },
    );
    try {
      await record.completion;
      if (!live()) return;
      if (commentDrafts.value[index]?.trim() === text)
        commentDrafts.value[index] = "";
    } catch {}
  }
  function reactPost(post) {
    return w.command(
      "动态",
      "reaction:" + post.id,
      {
        kind: post.liked ? "none" : "like",
        expectedRevision: post._source.revision,
        reason: "回应这条动态",
      },
      (body, key) =>
        w.services.momentsClient.reaction(post.id, {
          ...body,
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadBrowse("动态"),
        success: "回应已保存",
      },
    );
  }
  function playMedia(id) {
    const asset = w.state.media.find((a) => a.id === id);
    if (!asset || asset.type === "图片") return;
    playback?.pause();
    if (playing.value === id) {
      playing.value = null;
      return;
    }
    const media = document.createElement(
      asset.type === "语音" ? "audio" : "video",
    );
    media.src = asset.url;
    media.onended = () => {
      if (live()) playing.value = null;
    };
    media.ontimeupdate = () => {
      if (live()) {
        progress.value[id] = media.duration
          ? (media.currentTime / media.duration) * 100
          : 0;
        durations.value[id] = {
          current: media.currentTime,
          total: media.duration,
        };
      }
    };
    void media
      .play()
      .then(() => {
        if (!live()) {
          media.pause();
          return;
        }
        playing.value = id;
        playback = media;
      })
      .catch((error) => {
        if (live()) w.notify("播放失败：" + error.message);
      });
  }
  function mediaContent(asset, large = false) {
    if (mediaErrors.value[asset.id])
      return <small role="status">{"素材读取失败，请重新打开"}</small>;
    const MediaElement =
      asset.type === "图片" ? "img" : asset.type === "视频" ? "video" : "audio";
    if (!large && MediaElement !== "img")
      return (
        <span aria-label={asset.type}>
          {MediaElement === "audio" ? "▂ ▅ ▃ ▇ ▄ ▂ ▆" : "▶"}
        </span>
      );
    return (
      <MediaElement
        src={asset.url}
        alt={MediaElement === "img" ? asset.title : undefined}
        controls={MediaElement !== "img"}
        preload={MediaElement === "img" ? undefined : "metadata"}
        loading={MediaElement === "img" ? "lazy" : undefined}
        className="approved-media-content"
        onError={() => {
          if (live()) mediaErrors.value[asset.id] = true;
        }}
      />
    );
  }
  const mediaTime = (id) => {
    const times = durations.value[id];
    return times
      ? Math.floor(times.current) +
          " / " +
          (Number.isFinite(times.total) ? Math.floor(times.total) : "—") +
          " 秒"
      : "时长待读取";
  };
  lifecycle.onDispose(() => {
    active = false;
    playback?.pause();
    playback = null;
  });
  const errors = selector(() =>
    [...w.state.operations, ...w.state.diagnostics]
      .filter((r) => r.status === "error")
      .concat(pendingIssues(w.state.pending, w.state.characters)),
  );
  const peopleFilters = selector(() => [
    {
      key: "status",
      label: "状态",
      options: ["活跃", "暂未联系"],
    },
    {
      key: "tag",
      label: "标签",
      options: [
        ...new Set(character.value.people.map((p) => p.tag).filter(Boolean)),
      ],
    },
    {
      key: "scene",
      label: "场景",
      options: character.value.sites.map((s) => s.name),
    },
  ]);
  const mediaFilters = selector(() => [
    {
      key: "type",
      label: "类型",
      options: ["图片", "语音", "视频"],
    },
    {
      key: "tag",
      label: "标签",
      options: [...new Set(w.state.media.map((a) => a.tag).filter(Boolean))],
    },
    {
      key: "who",
      label: "角色",
      options: characters.value.map((c) => c.name),
    },
  ]);
  const bindings = {
    props,
    mediaContent,
    mediaTime,
    resources: selector(() => w.state.resources || []),
    openStudioResource: (id) =>
      w.navigatePath("/studio/edit/" + encodeURIComponent(id)),
    createStudioResource: (kind) =>
      w.navigatePath("/studio/edit/new?kind=" + encodeURIComponent(kind)),
    studioKind: (kind) =>
      ({
        world: "世界",
        place: "地点",
        item: "物品",
        outfit: "服装方案",
        npc: "人物",
        event_template: "事件",
        character_template: "角色模板",
        persona_fragment: "人格资料",
        visual_style: "视觉参考",
      })[kind] || "创作资源",
    createCharacter: () => w.navigatePath("/library/create"),
    character,
    commentsOpen,
    comment,
    commentDrafts,
    onCommentKeyDown: (event, index) => {
      if (event.key === "Enter" && !event.shiftKey && !isComposingKey(event)) {
        event.preventDefault();
        void comment(index);
      }
    },
    busy,
    commentTarget,
    ActionFeedback,
    FilterChips,
    search,
    filters,
    peopleFilters,
    people,
    selected,
    clear,
    person,
    mediaFilters,
    filteredMedia: media,
    playing,
    progress,
    allMedia: selector(() => w.state.media),
    libraryFilters: selector(() => [
      {
        key: "tag",
        label: "标签",
        options: [...new Set(characters.value.flatMap((c) => c.tags))],
      },
    ]),
    library,
    selectedId: selector({
      get: () => w.state.selectedId,
      set: (value) => w.select(value),
    }),
    characters,
    openNotificationOrCharacter: async (id) => {
      if (props.page === "通知与待处理") {
        const notice = w.state.notifications.find((n) => n.id === id);
        if (!notice) return;
        await w.services.operationsClient.readNotification(id);
        if (!live()) return;
        await w.loadBrowse("通知与待处理");
        if (!live()) return;
        const href = notificationPath(notice.link);
        if (href) w.navigatePath(href);
        else if (notice.fluctlight_id) {
          w.select(notice.fluctlight_id);
          w.navigate("聊天");
        }
        return;
      }
      w.select(id);
      w.navigate("聊天");
    },
    errors,
    openIssue: (id) => {
      const pending = w.state.pending.find((p) => p.id === id);
      const href = notificationPath(pending?.href);
      if (href) w.navigatePath(href);
      else {
        w.state.managementTab = "运行诊断";
        w.navigate("管理中心");
      }
    },
    reactPost,
    playMedia,
  };
  bindings.NotificationGroup = NotificationGroup;
  bindings.NotificationIssues = NotificationIssues;
  bindings.issueGroups = selector(() => notificationIssueGroups(errors.value));
  const render = () => <BrowsePageView view={bindings} />;
  return (context, cache) => (
    <Fragment>
      {render(context, cache)}
      {props.page === "动态" && !character.value.posts.length ? (
        <p className="muted approved-empty-feed">
          {"这里还没有动态，真实生活里的片刻会慢慢留下来。"}
        </p>
      ) : null}
    </Fragment>
  );
}
/** @param {{page: string, onGo?: (page: string) => void}} input */
export default function BrowsePage(input) {
  return useController(createBrowsePageController, {
    ...input,
  });
}

import { defineComponent, ref, computed, onUnmounted, h, Fragment } from "vue";
import { pendingIssues } from "../presentation.mjs";
import { useWorkspace } from "../workspace.mjs";
import { renderBrowsePage } from "../renderers/BrowsePage.mjs";
import { FilterChips, ActionFeedback } from "../shared/components.mjs";
export default defineComponent({
  name: "BrowsePage",
  props: { page: { type: String, required: true } },
  setup(props) {
    const w = useWorkspace(),
      character = w.current,
      playing = ref(null),
      progress = ref({}),
      search = ref(""),
      filters = ref({
        status: "",
        tag: "",
        scene: "",
        type: "",
        who: props.page === "媒体" ? character.value.name : "",
      }),
      selected = ref(null),
      commentDrafts = ref({}),
      commentsOpen = ref({}),
      commentTarget = ref(null);
    let playback = null;
    const durations = ref({}),
      mediaErrors = ref({}),
      scope = w.currentScope();
    let active = true;
    const live = () => active && w.currentScope() === scope;
    const busy = computed(() =>
      w.state.operations.some(
        (r) =>
          r.fid === character.value.id &&
          r.label === "动态" &&
          r.status === "pending",
      ),
    );
    const matches = (text) =>
      String(text).toLowerCase().includes(search.value.trim().toLowerCase());
    const people = computed(() =>
      character.value.people.filter(
        (p) =>
          matches(p.name + " " + p.identity) &&
          (!filters.value.status || filters.value.status === p.status) &&
          (!filters.value.tag || filters.value.tag === p.tag) &&
          (!filters.value.scene || filters.value.scene === p.location),
      ),
    );
    const person = computed(() =>
      character.value.people.find((p) => p.id === selected.value),
    );
    const characters = computed(() =>
      props.page === "通知与待处理"
        ? w.state.notifications
            .filter((n) => !n.read_at)
            .map((n) => {
              const c =
                w.state.characters.find((v) => v.id === n.fluctlight_id) || {};
              return {
                id: n.id,
                name: c.name || n.title,
                initial: c.initial || "光",
                color: c.color || "#e4ede4",
                noticeTitle: n.title,
                unreadCount: n.read_at ? 0 : 1,
                messages: [{ text: n.body || n.title, user: false }],
                _notice: n,
              };
            })
        : w.state.characters,
    );
    const library = computed(() =>
      characters.value.filter(
        (c) =>
          matches(c.name + " " + c.job + " " + c.bio) &&
          (!filters.value.tag || c.tags.includes(filters.value.tag)),
      ),
    );
    const media = computed(() =>
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
      filters.value = { status: "", tag: "", scene: "", type: "", who: "" };
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
        { onSaved: () => w.loadBrowse("动态"), success: "评论已保存" },
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
        { onSaved: () => w.loadBrowse("动态"), success: "回应已保存" },
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
        return h("small", { role: "status" }, "素材读取失败，请重新打开");
      const tag =
        asset.type === "图片"
          ? "img"
          : asset.type === "视频"
            ? "video"
            : "audio";
      if (!large && tag !== "img")
        return h(
          "span",
          { "aria-label": asset.type },
          tag === "audio" ? "▂ ▅ ▃ ▇ ▄ ▂ ▆" : "▶",
        );
      return h(tag, {
        src: asset.url,
        alt: tag === "img" ? asset.title : undefined,
        controls: tag !== "img",
        preload: tag === "img" ? undefined : "metadata",
        loading: tag === "img" ? "lazy" : undefined,
        class: "approved-media-content",
        onError: () => {
          if (live()) mediaErrors.value[asset.id] = true;
        },
      });
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
    onUnmounted(() => {
      active = false;
      playback?.pause();
      playback = null;
    });
    const errors = computed(() =>
      [...w.state.operations, ...w.state.diagnostics]
        .filter((r) => r.status === "error")
        .concat(pendingIssues(w.state.pending, w.state.characters)),
    );
    const peopleFilters = computed(() => [
      { key: "status", label: "状态", options: ["活跃", "暂未联系"] },
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
    const mediaFilters = computed(() => [
      { key: "type", label: "类型", options: ["图片", "语音", "视频"] },
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
      openStudio: () => w.navigatePath("/studio?legacy=1"),
      createCharacter: () => w.navigatePath("/library/create"),
      character,
      w: commentsOpen,
      q: comment,
      C: commentDrafts,
      ue: (event, index) => {
        if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
          event.preventDefault();
          void comment(index);
        }
      },
      t: busy,
      ne: commentTarget,
      ActionFeedback,
      FilterChips,
      f: search,
      c: filters,
      A: peopleFilters,
      I: people,
      d: selected,
      se: clear,
      K: person,
      B: mediaFilters,
      H: media,
      s: playing,
      n: progress,
      media: computed(() => w.state.media),
      te: computed(() => [
        {
          key: "tag",
          label: "标签",
          options: [...new Set(characters.value.flatMap((c) => c.tags))],
        },
      ]),
      le: library,
      selectedId: computed({
        get: () => w.state.selectedId,
        set: (value) => w.select(value),
      }),
      characters,
      a: async (id) => {
        if (props.page === "通知与待处理") {
          const notice = w.state.notifications.find((n) => n.id === id);
          if (!notice) return;
          await w.services.operationsClient.readNotification(id);
          if (!live()) return;
          await w.loadBrowse("通知与待处理");
          if (!live()) return;
          const href = notice.link;
          if (
            typeof href === "string" &&
            href.startsWith("/") &&
            !href.startsWith("//")
          )
            w.navigatePath(href);
          else if (notice.fluctlight_id) {
            w.select(notice.fluctlight_id);
            w.navigate("聊天");
          }
          return;
        }
        w.select(id);
        w.navigate("聊天");
      },
      i: errors,
      r: (id) => {
        const pending = w.state.pending.find((p) => p.id === id);
        if (pending?.href?.startsWith("/") && !pending.href.startsWith("//"))
          w.navigatePath(pending.href);
        else {
          w.state.managementTab = "运行诊断";
          w.navigate("管理中心");
        }
      },
      reactPost,
      playMedia,
    };
    const render = renderBrowsePage(bindings);
    return (context, cache) =>
      h(Fragment, [
        render(context, cache),
        props.page === "动态" && !character.value.posts.length
          ? h(
              "p",
              { class: "muted approved-empty-feed" },
              "这里还没有动态，真实生活里的片刻会慢慢留下来。",
            )
          : null,
      ]);
  },
});

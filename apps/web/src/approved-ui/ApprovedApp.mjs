import {
  defineComponent,
  ref,
  computed,
  provide,
  watch,
  onMounted,
  onUnmounted,
  nextTick,
  h,
  Fragment,
} from "vue";
import { createWorkspace, WorkspaceKey } from "./workspace.mjs";
import { approvedRoute, approvedHref, lifePages } from "./routing.mjs";
import { renderApp } from "./renderers/App.mjs";
import ChatPage from "./pages/ChatPage.mjs";
import LifePage from "./pages/LifePage.mjs";
import WardrobePage from "./pages/WardrobePage.mjs";
import WorldPage from "./pages/WorldPage.mjs";
import BrowsePage from "./pages/BrowsePage.mjs";
import ManagementPage from "./pages/ManagementPage.mjs";
import CharacterAvatar from "./shared/CharacterAvatar.vue";
import LibraryPage from "./pages/LibraryPage.vue";
import CharacterWizardPage from "./pages/CharacterWizardPage.vue";
import CharacterDetailPage from "./pages/CharacterDetailPage.vue";
import DomainPage from './pages/DomainPage.vue';
import AuthPage from './pages/AuthPage.vue';
import StudioPage from './pages/StudioPage.vue';
import MediaPage from './pages/MediaPage.vue';
import InformationPage from './pages/InformationPage.vue';
import { chatSummaryPreference } from "../v3/chat-summary-preference.mjs";
import { Wu, Co, sc, qu, zu, Gu, nc, ec } from "./shared/icons.mjs";
export default defineComponent({
  name: "ApprovedApp",
  setup() {
    const workspace = createWorkspace(),
      state = workspace.state,
      route = ref(approvedRoute(location.hash));
    provide(WorkspaceKey, workspace);
    const mobileNavOpen=ref(false);
    const sidebarOpen = ref(true),
      switchOpen = ref(false),
      preview = ref(true),
      main = ref(),
      simulate = ref(false);
    const page = computed(() => route.value.page || state.page),
      selectedId = computed({
        get: () => state.selectedId,
        set: (value) => {
          workspace.select(value);
          navigate(page.value);
        },
      });
    function navigate(page) {
      location.hash = approvedHref(page, state.selectedId);
    }
    function togglePreview(){
      preview.value=!preview.value;
      if(state.session&&state.selectedId)chatSummaryPreference({owner:state.session.actorId,fid:state.selectedId,defaultFolded:false}).save(!preview.value);
    }
    workspace.navigate = navigate;
    workspace.navigatePath = (path) => (location.hash = "#" + path);
    const parse = () => {
      route.value = approvedRoute(location.hash);
    };
    async function syncRoute() {
      if (!state.ready || !state.session) return;
      const value = route.value;
      mobileNavOpen.value=false;
      if(value.auth){navigate('角色库');return;}
      if (value.fid && state.characters.some((c) => c.id === value.fid))
        workspace.select(value.fid);
      if (
        !state.selectedId &&
        ["聊天", ...lifePages, "人物关系"].includes(value.page)
      ) {
        navigate("角色库");
        return;
      }
      state.page = value.page;
      state.managementSection = value.query.get("section") || "";
      if (value.diagnostics) state.managementTab = "运行诊断";
      state.feedScope = value.query.get("fid") || null;
      state.route=value;
      const preference = chatSummaryPreference({
        owner: state.session.actorId,
        fid: state.selectedId,
        defaultFolded: false,
      });
      preview.value = matchMedia('(max-width:800px)').matches?false:!preference.read();
      if(!value.feature&&!value.notFound&&!['创作中心','媒体'].includes(value.page)&&!(value.page==='世界'&&!state.selectedId))await workspace.load(value.page);
      await nextTick();
      main.value?.scrollTo({ top: 0 });
    }
    const stop = watch(route, () => void syncRoute());
    const narrowScreen=matchMedia('(max-width:800px)');
    const resizeWorkspace=()=>{mobileNavOpen.value=false;if(narrowScreen.matches)preview.value=false;};
    const closeMenus = (event) => {
      const target = event.target;
      document
        .querySelectorAll("details.actions-menu[open]")
        .forEach((details) => {
          if (!details.contains(target) || target.closest("button"))
            details.open = false;
        });
    };
    const wheel = (event) => {
      const target = event.target instanceof Element ? event.target : null;
      if (
        !target ||
        event.ctrlKey ||
        !event.deltaY ||
        target.closest("select,input,textarea,option,datalist")
      )
        return;
      const list = target.closest(".scroll-lock-list,[data-scroll-lock]");
      if (!list) return;
      const style = getComputedStyle(list),
        scrollable =
          list.scrollHeight > list.clientHeight &&
          list.clientHeight > 0 &&
          ["auto", "scroll"].includes(style.overflowY);
      list.style.overscrollBehavior = scrollable ? "contain" : "auto";
      if (
        scrollable &&
        ((event.deltaY < 0 && list.scrollTop <= 0) ||
          (event.deltaY > 0 &&
            list.scrollTop + list.clientHeight >= list.scrollHeight))
      )
        event.preventDefault();
    };
    const render = renderApp({
      d: navigate,
      u: switchOpen,
      character: computed(()=>page.value==="初始化角色"?{...workspace.current.value,name:"创建角色",initial:"+",color:"#dce9e2"}:workspace.current.value.id?workspace.current.value:{...workspace.current.value,name:'我的空间',initial:'我'}),
      mobileNavOpen,
      toggleNavigation:()=>mobileNavOpen.value=!mobileNavOpen.value,
      detailRoute:computed(()=>route.value.feature?route.value:null),
      information:computed(()=>route.value.information),InformationPage,
      unavailableRoute:computed(()=>route.value.notFound||route.value.fid&&!state.characters.some(c=>c.id===route.value.fid)),
      DomainPage,StudioPage,MediaPage,
      simulateFailure: simulate,
      characters: computed(() => state.characters),
      selectedId,
      r: page,
      a: sidebarOpen,
      t: lifePages,
      s: [
        ["世界", Wu],
        ["动态", Co],
        ["人物关系", sc],
        ["媒体", qu],
      ],
      n: [
        ["通知与待处理", zu],
        ["角色库", Gu],
        ["创作中心", nc],
        ["管理中心", ec],
      ],
      c: main,
      f: preview,
      togglePreview,
      ChatPage,
      WardrobePage,
      WorldPage,
      LifePage,
      ManagementPage,
      BrowsePage,
      CharacterAvatar,
      LibraryPage,
      CharacterWizardPage,
      CharacterDetailPage,
      toast: computed(() => state.toast),
      outfitTab: computed({
        get: () => state.outfitTab,
        set: (value) => (state.outfitTab = value),
      }),
      isDemo: false,
    });
    onMounted(async () => {
      window.addEventListener("hashchange", parse);
      document.addEventListener("click", closeMenus);
      narrowScreen.addEventListener('change',resizeWorkspace);
      document.addEventListener("wheel", wheel, {
        capture: true,
        passive: false,
      });
      try {
        await workspace.bootstrap();
        if (!state.session) {
          if (!location.hash || !route.value.auth) location.hash = "#/auth";
          return;
        }
        await syncRoute();
      } catch {
        /* Explicit bootstrap failure is rendered with a retry path. */
      }
    });
    onUnmounted(() => {
      stop();
      workspace.dispose();
      window.removeEventListener("hashchange", parse);
      narrowScreen.removeEventListener('change',resizeWorkspace);
      document.removeEventListener("click", closeMenus);
      document.removeEventListener("wheel", wheel, { capture: true });
    });
    return (context, cache) => {
      if (!state.ready)
        return h(
          "div",
          { class: "approved-bootstrap", role: "status" },
          "正在读取你的角色与工作区…",
        );
      if (state.errors.bootstrap)
        return h(
          "section",
          { class: "card approved-bootstrap", role: "alert" },
          [
            h("p", state.errors.bootstrap),
            h(
              "button",
              { class: "button", onClick: () => location.reload() },
              "重新读取",
            ),
          ],
        );
      if(!state.session)return h(AuthPage);
      const error = Object.values(state.errors).filter(Boolean).at(-1);
      return h(Fragment, [
        render(context, cache),
        error
          ? h("div", { class: "approved-read-feedback", role: "alert" }, [
              h("span", error),
              h(
                "button",
                {
                  class: "text-button",
                  onClick: () => workspace.load(page.value),
                },
                "重新读取",
              ),
            ])
          : null,
      ]);
    };
  },
});

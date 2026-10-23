import { defineComponent, ref, computed } from "vue";
import { useWorkspace } from "../workspace.mjs";
import { renderWorldPage } from "../renderers/WorldPage.mjs";
import { DetailDrawer, ActionFeedback } from "../shared/components.mjs";
import { worldDefinitionError } from "../../v3/proto/components/world-definition.js";
import { stagedCommand } from "../staged-command.mjs";
import { randomId } from "../../random-id.mjs";
const clone = (v) => JSON.parse(JSON.stringify(v));
export default defineComponent({
  name: "WorldPage",
  setup() {
    const w = useWorkspace(),
      worlds = computed(() => w.state.worlds),
      character = w.current,
      selected = ref(null),
      world = computed(() => worlds.value.find((v) => v.id === selected.value)),
      expanded = ref(false),
      instruction = ref(""),
      place = ref(null),
      siteIndex = ref(0),
      siteName = ref(""),
      siteDialog = ref(false),
      worldDraft = ref(null),
      removeId = ref(null),
      operation = ref("");
    const busy = computed(() =>
      w.state.operations.some(
        (r) =>
          r.fid === character.value.id &&
          r.label === "世界" &&
          r.status === "pending",
      ),
    );
    const fields = [
      ["alias", "别名"],
      ["floor", "楼层"],
      ["description", "描述"],
      ["atmosphere", "氛围"],
      ["furniture", "固定陈设"],
      ["layout", "布局描述"],
    ];
    function newWorld() {
      worldDraft.value = { id: null, name: "", genre: "" };
    }
    async function saveWorld() {
      const draft = worldDraft.value,
        submitted = JSON.stringify(worldDraft.value);
      if (!draft?.name.trim() || busy.value) return;
      const fid = character.value.id,
        source = draft._source;
      operation.value = "world";
      const body = {
        name: draft.name.trim(),
        flavor: draft.genre,
        reason: source ? "编辑世界" : "创建世界",
        ...(source ? { expectedRevision: source.revision } : {}),
      };
      const record = w.command(
        "世界",
        "world:" + String(source?.id || "new"),
        body,
        (value, key) =>
          source
            ? w.client.updateCatalogWorld(fid, source.id, {
                ...value,
                idempotencyKey: key,
              })
            : w.client.createCatalogWorld(fid, {
                ...value,
                idempotencyKey: key,
              }),
        { onSaved: () => w.loadWorlds(), success: "世界已保存" },
      );
      try {
        const result = await record.completion;
        selected.value = result.id;
        expanded.value = true;
        if (JSON.stringify(worldDraft.value) === submitted)
          worldDraft.value = null;
      } catch {}
    }
    function editPlace(value, index) {
      siteIndex.value = index;
      place.value = clone(value);
    }
    async function savePlace() {
      const submitted = JSON.stringify(place.value);
      const draft = place.value,
        currentWorld = world.value;
      if (!draft?.name.trim() || !currentWorld || busy.value) return;
      const fid = character.value.id,
        source = draft._source,
        site = currentWorld.sites[siteIndex.value];
      const body = {
        worldId: currentWorld.id,
        expectedRevision: source?.revision || source?.version || 0,
        name: source?.name || draft.name.trim(),
        description: draft.description,
        alias: draft.alias || "",
        group: site?.name || "",
        ...(site?.id &&
        !String(site.id).startsWith("draft:") &&
        site.id !== "unclassified" &&
        !String(site.id).startsWith("group:")
          ? { venueCategoryId: site.id }
          : {}),
        floor: draft.floor || "",
        atmosphere: draft.atmosphere || "",
        furniture:
          source && draft.furniture === source.furniture
            ? (source._rawCollections?.furniture ?? draft.furniture)
            : draft.furniture || "",
        layout: draft.layout || "",
        reason: "保存世界中的具体地点",
      };
      operation.value = "place";
      const record = w.command(
        "世界",
        "place:" + String(source?.id || "new:" + currentWorld.id),
        body,
        (value, key) =>
          source
            ? w.client.updateWorldPlace(fid, source.id, {
                ...value,
                idempotencyKey: key,
              })
            : w.client.createWorldPlace(fid, { ...value, idempotencyKey: key }),
        { onSaved: () => w.loadWorlds(), success: "地点已保存" },
      );
      try {
        await record.completion;
        if (JSON.stringify(place.value) === submitted) place.value = null;
      } catch {}
    }
    function addSite() {
      const name = siteName.value.trim();
      if (!world.value || !name) return;
      if (world.value.sites.some((s) => s.name === name)) {
        w.notify("已有同名场地");
        return;
      }
      world.value.sites.push({
        id: "draft:" + randomId(),
        name,
        places: [],
        draft: true,
      });
      siteName.value = "";
      siteDialog.value = false;
      w.notify("场地已加入本次草稿，保存第一个地点后形成实际分组");
    }
    async function expandWorld() {
      const target = world.value;
      if (!target || busy.value) return;
      const fid = character.value.id,
        owner = w.state.session?.actorId,
        source = target._source;
      operation.value = "expand";
      const input = {
        wid: target.id,
        world: source,
        instruction:
          instruction.value.trim() ||
          "按当前世界主题补充合理的场地分类与具体地点，保留现有地点及历史，不改变角色位置。",
      };
      const record = w.command(
        "世界",
        "expand:" + target.id,
        input,
        async (value, key) => {
          const live = () => w.isCurrent(owner, fid),
            journal = stagedCommand({
              storage: sessionStorage,
              key: "fluctlight:approved-stages:" + key,
              live,
            });
          const body = {
            background: value.world.background || value.world.description || "",
            rules: value.world.rules || "",
            description: value.world.description || "",
            flavor: value.world.flavor || "",
          };
          const resource = await journal.stage(
            "source",
            () => ({
              kind: "world",
              name: value.world.name,
              body,
              reason: "为原世界准备扩写底稿",
            }),
            (payload, idempotencyKey) =>
              w.services.studioClient.save(null, {
                ...payload,
                idempotencyKey,
              }),
          );
          const accepted = await journal.stage(
            "job",
            () => ({
              resourceId: resource.id,
              resourceRevision: resource.revision,
              kind: "assistant",
              name: resource.name,
              body,
              instruction: value.instruction,
              reason: "扩写已有世界中的场地和地点",
            }),
            (payload, idempotencyKey) =>
              w.services.studioClient.createJob({ ...payload, idempotencyKey }),
          );
          let job = accepted;
          for (let attempt = 0; attempt < 100; attempt++) {
            if (!live()) throw Error("角色已变化，请回到原世界继续");
            if (document.hidden)
              throw Error("已暂停后台观察，返回后可重试读取原扩写任务");
            job = await w.services.studioClient.job(accepted.id);
            if (!live()) throw Error("角色已变化");
            if (!["queued", "running"].includes(job.status)) break;
            await new Promise((resolve) => setTimeout(resolve, 3000));
          }
          if (
            job.status !== "completed" ||
            job.result?.disposition !== "proposal" ||
            !job.result.body?.framework
          )
            throw Error(job.error_code || "扩写仍未完成，请查看原创作任务");
          const proposal = { ...body, framework: job.result.body.framework },
            invalid = worldDefinitionError(proposal);
          if (invalid) throw Error(invalid);
          const saved = await journal.stage(
            "save",
            () => ({
              resourceId: resource.id,
              kind: "world",
              name: resource.name,
              body: proposal,
              expectedVersion: resource.draft_version,
              reason: "保存实际模型返回的世界扩写",
            }),
            ({ resourceId, ...payload }, idempotencyKey) =>
              w.services.studioClient.save(resourceId, {
                ...payload,
                idempotencyKey,
              }),
          );
          const published = await journal.stage(
            "publish",
            () => ({
              resourceId: saved.id,
              expectedRevision: saved.revision,
              reason: "发布实际世界扩写",
            }),
            ({ resourceId, ...payload }, idempotencyKey) =>
              w.client.publishStudioResource(resourceId, {
                ...payload,
                idempotencyKey,
              }),
          );
          const result = await journal.stage(
            "apply",
            async () => {
              const preview = await w.services.studioClient.preview(
                fid,
                published.id,
                published.published_version,
                value.wid,
              );
              return {
                resourceId: published.id,
                fluctlightId: fid,
                runtimeWorldId: value.wid,
                resourceVersion: published.published_version,
                expectedInstanceRevision: preview.instance_revision,
                expectedRuntimeRevision: preview.runtime_revision,
                choices: preview.diff.map((row) => ({
                  field: row.field,
                  decision: row.field === "framework" ? "override" : "keep",
                })),
                reason: "应用明确请求的世界扩写，保留已有地点与历史",
              };
            },
            ({ resourceId, ...payload }, idempotencyKey) =>
              w.services.studioClient.apply(resourceId, {
                ...payload,
                idempotencyKey,
              }),
          );
          journal.finish();
          return result;
        },
        {
          onSaved: () => w.loadWorlds(),
          success: "扩写已应用并读回实际地点",
          multiStage: true,
        },
      );
      try {
        await record.completion;
      } catch {}
    }

    function removeWorld(id) {
      const value = worlds.value.find((v) => v.id === id);
      if (!value) return;
      const fid = character.value.id;
      const record = w.command(
        "世界",
        "delete-world:" + id,
        {
          expectedRevision: value._source.revision,
          reason: "移除不再使用且没有引用的世界",
        },
        (body, key) =>
          w.client.deleteCatalogWorld(fid, id, {
            ...body,
            idempotencyKey: key,
          }),
        { onSaved: () => w.loadWorlds(), success: "世界已移除" },
      );
      record.completion
        .then(() => {
          removeId.value = null;
          if (selected.value === id) selected.value = null;
        })
        .catch(() => {});
    }
    return renderWorldPage({
      s: world,
      t: selected,
      u: siteDialog,
      c: worldDraft,
      n: expanded,
      l: instruction,
      f: busy,
      te: expandWorld,
      C: operation,
      r: siteIndex,
      i: place,
      se: editPlace,
      w: newWorld,
      worlds,
      character,
      characters: computed(() => w.state.characters),
      d: removeId,
      DetailDrawer,
      ne: savePlace,
      ActionFeedback,
      A: fields,
      B: saveWorld,
      ue: addSite,
      a: siteName,
      removeWorld,
    });
  },
});

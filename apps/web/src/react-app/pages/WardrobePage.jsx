import { useController } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { Fragment } from "@/react-app/render-helpers";
import { slots } from "@/state/workspace.mjs";
import WardrobePreparation from "@/react-app/shared/WardrobePreparation";
import { WardrobePageView } from "@/react-app/views/WardrobePage";
import { GarmentArt, ActionFeedback } from "@/react-app/components";
const copy = (value) => JSON.parse(JSON.stringify(value));
const slotMap = {
  外套: "outer",
  上衣: "upper_body",
  内衣: "underwear",
  内裤: "panties",
  下衣: "lower_body",
  袜子: "socks",
  鞋子: "shoes",
  配饰: "accessory",
};
const seasons = {
  春: "spring",
  夏: "summer",
  秋: "autumn",
  冬: "winter",
};
function createWardrobePageController(props, context, lifecycle) {
  const w = lifecycle.workspace,
    character = w.current,
    { inventoryClient } = w.services;
  const action = lifecycle.cell(""),
    selectedOutfit = lifecycle.cell(null),
    busy = selector(() =>
      w.state.operations.some(
        (r) =>
          r.fid === character.value.id &&
          ["衣柜", "换装"].includes(r.label) &&
          (r.status === "pending" || r.requestUnresolved),
      ),
    );
  const tab = selector({
      get: () => w.state.outfitTab,
      set: (v) => (w.state.outfitTab = v),
    }),
    tag = lifecycle.cell(character.value.outfitTags[0] || ""),
    name = lifecycle.cell(""),
    items = lifecycle.cell(copy(character.value.wearing)),
    editingId = lifecycle.cell(null),
    preview = lifecycle.cell(null),
    newTag = lifecycle.cell(""),
    renameTag = lifecycle.cell(""),
    type = lifecycle.cell(""),
    season = lifecycle.cell(""),
    renameOutfit = lifecycle.cell(null),
    renameValue = lifecycle.cell(""),
    confirmation = lifecycle.cell(null),
    garment = lifecycle.cell(null);
  lifecycle.observe(
    () => character.value._wardrobe,
    (value) => {
      if (
        value &&
        !name.value &&
        !editingId.value &&
        !Object.values(items.value).some((list) => list.length)
      )
        items.value = copy(character.value.wearing);
    },
    {
      immediate: true,
    },
  );
  const duplicate = selector(() =>
    character.value.outfits.some(
      (o) => o.name === name.value.trim() && o.id !== editingId.value,
    ),
  );
  lifecycle.observe(
    () => character.value.outfitTags,
    (values) => {
      if (!values.includes(tag.value)) tag.value = values[0] || "";
    },
    {
      immediate: true,
    },
  );
  const normalize = (value) =>
    Object.fromEntries(
      slots.map((s) => [
        s,
        (value[s] || []).filter((id) =>
          character.value.garments.some((g) => g.id === id && g.type === s),
        ),
      ]),
    );
  const summarize = (outfit) =>
    slots
      .map(
        (s) =>
          s +
          "：" +
          ((outfit.items[s] || [])
            .map(
              (id) => character.value.garments.find((g) => g.id === id)?.name,
            )
            .filter(Boolean)
            .join("、") || "未选"),
      )
      .join(" / ");
  const choose = (slot, id, multiple = false) => {
    if (!tag.value) return;
    const before = items.value[slot] || [];
    items.value[slot] = multiple
      ? before.includes(id)
        ? before.filter((v) => v !== id)
        : [...before, id]
      : before.includes(id)
        ? []
        : [id];
  };
  const confirm = (title, message, run) =>
    (confirmation.value = {
      title,
      message,
      run,
    });
  async function saveOutfit() {
    if (busy.value || duplicate.value || !name.value.trim() || !tag.value)
      return;
    const fid = character.value.id,
      state = character.value._wardrobe,
      old = character.value.outfits.find((o) => o.id === editingId.value),
      body = {
        name: name.value.trim(),
        tags: [tag.value],
        itemIds: Object.values(normalize(items.value)).flat(),
        profileId: state.library.profile_id,
        expectedRevision: state.wearing.revision,
        expectedOutfitRevision: old ? old._source.revision : 0,
        reason: "保存穿搭方案",
      };
    const submitted = {
      name: name.value,
      items: copy(items.value),
      tag: tag.value,
      editingId: editingId.value,
    };
    action.value = "save";
    const record = w.command(
      "衣柜",
      "outfit:" + String(editingId.value || "new"),
      {
        id: editingId.value,
        body,
      },
      (value, key) =>
        inventoryClient.saveOutfit(fid, value.id, {
          ...value.body,
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadWardrobe(),
        success: "搭配方案已保存",
      },
    );
    try {
      await record.completion;
      if (
        name.value === submitted.name &&
        tag.value === submitted.tag &&
        editingId.value === submitted.editingId &&
        JSON.stringify(items.value) === JSON.stringify(submitted.items)
      ) {
        name.value = "";
        items.value = {};
        editingId.value = null;
      }
    } catch {}
  }
  function edit(outfit) {
    editingId.value = outfit.id;
    name.value = outfit.name;
    tag.value = outfit.tag;
    items.value = copy(outfit.items);
    tab.value = "为她搭配";
  }
  function wear(outfit) {
    if (busy.value) return;
    const fid = character.value.id,
      state = character.value._wardrobe;
    action.value = "wear";
    selectedOutfit.value = outfit.id;
    return w.command(
      "换装",
      "wear-outfit:" + outfit.id,
      {
        outfitId: outfit.id,
        expectedOutfitRevision: outfit._source.revision,
        profileId: state.library.profile_id,
        expectedRevision: state.wearing.revision,
        kind: "outfit",
        reason: "想请你试试这套搭配",
      },
      (body, key) =>
        inventoryClient.action(fid, {
          ...body,
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadWardrobe(),
        success: "换装请求已受理",
      },
    );
  }
  async function rename(outfit) {
    const value = renameValue.value.trim(),
      fid = character.value.id;
    if (!value || busy.value) return;
    const state = character.value._wardrobe;
    action.value = "rename";
    const record = w.command(
      "衣柜",
      "rename:" + outfit.id,
      {
        name: value,
        profileId: state.library.profile_id,
        expectedOutfitRevision: outfit._source.revision,
        expectedRevision: state.wearing.revision,
        reason: "修改方案名称",
      },
      (body, key) =>
        inventoryClient.renameOutfit(fid, outfit.id, {
          ...body,
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadWardrobe(),
        success: "方案名称已保存",
      },
    );
    try {
      await record.completion;
      if (
        renameOutfit.value === outfit.id &&
        renameValue.value.trim() === value
      )
        renameOutfit.value = null;
    } catch {}
  }
  async function saveGarment() {
    const value = garment.value,
      submitted = JSON.stringify(garment.value);
    if (!value?.name.trim() || busy.value) return;
    const fid = character.value.id,
      source = value._source,
      state = character.value._wardrobe,
      sameType =
        !!source &&
        character.value.garments.find((g) => g.id === source.id)?.type ===
          value.type,
      slot = sameType ? source.slot : slotMap[value.type],
      body = {
        expectedRevision: state.wearing.revision,
        reason: source ? "更正服装登记" : "补录已知服装",
        ...(source
          ? {
              expectedItemRevision: source.revision,
            }
          : {}),
        item: {
          itemKind: "wearable",
          category: sameType ? source.category : slot,
          slot,
          description: value.name.trim(),
          ownership: source?.ownership || "unknown",
          availability: source?.availability || "available",
          clothingType: sameType ? source.clothing_type || source.slot : slot,
          seasons: value.seasons.map((s) => seasons[s]).filter(Boolean),
        },
      };
    action.value = "garment";
    const record = w.command(
      "衣柜",
      "garment:" + String(source?.id || "new"),
      body,
      (payload, key) =>
        source
          ? inventoryClient.update(fid, source.id, {
              ...payload,
              idempotencyKey: key,
            })
          : inventoryClient.create(fid, {
              ...payload,
              idempotencyKey: key,
            }),
      {
        onSaved: () => w.loadWardrobe(),
        success: "服装登记已保存",
      },
    );
    try {
      await record.completion;
      if (JSON.stringify(garment.value) === submitted) garment.value = null;
    } catch {}
  }
  function removeGarment(id) {
    const value = character.value.garments.find((g) => g.id === id);
    if (!value) return;
    confirm("归档衣服", "将从当前衣柜移出，历史和方案引用会保留。", () => {
      const fid = character.value.id;
      w.command(
        "衣柜",
        "archive:" + id,
        {
          expectedRevision: character.value._wardrobe.wearing.revision,
          expectedItemRevision: value._source.revision,
          reason: "归档当前服装登记",
        },
        (body, key) =>
          inventoryClient.archive(fid, id, {
            ...body,
            idempotencyKey: key,
          }),
        {
          onSaved: () => w.loadWardrobe(),
          success: "服装已归档",
        },
      );
    });
  }
  async function saveTags(remove) {
    if (busy.value) return;
    const fid = character.value.id,
      old = renameTag.value,
      label = newTag.value.trim(),
      tags = remove
        ? character.value.outfitTags.filter((t) => t !== remove)
        : old
          ? character.value.outfitTags.map((t) => (t === old ? label : t))
          : [...character.value.outfitTags, label];
    if (!remove && !label) return;
    const record = w.command(
      "衣柜",
      "tags",
      {
        tags: [...new Set(tags)],
        renames:
          !remove && old && label
            ? {
                [old]: label,
              }
            : {},
        expectedRevision: character.value._wardrobe.tags.revision,
        reason: "维护穿搭场景标签",
      },
      (body, key) =>
        inventoryClient.saveTags(fid, {
          ...body,
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadWardrobe(),
        success: "标签已保存",
      },
    );
    try {
      await record.completion;
      if (!remove && newTag.value.trim() === label && renameTag.value === old) {
        newTag.value = "";
        renameTag.value = "";
      }
    } catch {}
  }
  const render = () => (
    <WardrobePageView
      view={{
        character,
        tab,
        tag,
        items,
        choose,
        summarize,
        name,
        duplicate,
        saveOutfit,
        busy,
        action,
        editingId,
        renameTag,
        newTag,
        removeTag: (value) =>
          confirm("删除标签", "删除「" + value + "」的分类，保留原方案。", () =>
            saveTags(value),
          ),
        saveTags: () => saveTags(),
        renameOutfit,
        rename,
        renameValue,
        preview,
        edit,
        confirm,
        wear,
        selectedOutfit,
        type,
        season,
        garment,
        filteredGarments: selector(() =>
          character.value.garments.filter(
            (g) =>
              (!type.value || g.type === type.value) &&
              (!season.value || g.seasons.includes(season.value)),
          ),
        ),
        GarmentArt,
        removeGarment,
        saveGarment,
        ActionFeedback,
        confirmation,
      }}
    />
  );
  return (context, cache) => (
    <Fragment>
      <WardrobePreparation />
      {render(context, cache)}
    </Fragment>
  );
}
export default function WardrobePage(input = {}) {
  return useController(createWardrobePageController, {
    ...input,
  });
}

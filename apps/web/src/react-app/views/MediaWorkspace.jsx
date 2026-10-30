import { Eye, Upload, Sparkles, Search, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Switch } from "@/components/ui/switch";
import { Surface, ChoiceSelect, FieldInput, Disclosure } from "../controls";
import {EmptyState} from "../shared/Presentation";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import MediaUpload from "../shared/MediaUpload";
import { MediaReader, MediaMore, SavedMedia } from "../shared/MediaParts";
import MediaDetail from "../shared/MediaDetail";
import VisualWorkspace from "../shared/VisualWorkspace";
import MediaGeneration from "../shared/MediaGeneration";
import AvatarWorkspace from "../shared/AvatarWorkspace";
import { mediaSource, mediaKind } from "@/approved-ui/media-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import {compactDateTime} from '@/approved-ui/presentation.mjs';
export default function MediaWorkspaceView({ model: m }) {
  return (
    <div className="replica-media resource-page">
      <WorkspacePageHeader
        eyebrow="MEDIA / MOMENTS IN PICTURES"
        title="把生活，留在画面里"
        description={
          m.route.fid || m.route.query?.get("fid")
            ? (m.actor?.name || "当前角色") + "的影像、灵感与视觉身份。"
            : "你的影像、灵感与视觉身份。"
        }
        actions={
          <>
            {m.tab !== "gallery" && !m.route.fid && !m.route.query?.get('fid') && (
              <ChoiceSelect
                aria-label="媒体操作角色"
                value={m.target}
                onChange={(event) => m.setTarget(event.target.value)}
              >
                <option value="">请选择角色</option>
                {m.w.state.characters.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </ChoiceSelect>
            )}
            {m.tab === 'gallery' && <Button
              size="icon"
              variant="ghost"
              aria-label="刷新媒体库"
              onClick={m.library.load}
            >
              <RefreshCw size={15} />
            </Button>}
            <Button
              variant="outline"
              onClick={(event) => {
                m.trigger.current = event.currentTarget;
                m.setUploadOpen(true);
              }}
            >
              <Upload size={15} />
              添加素材
            </Button>
            <Button disabled={!m.target} onClick={() => m.setTab("generate")}>
              <Sparkles size={15} />
              生成画面
            </Button>
          </>
        }
      />
      <Tabs value={m.tab} onValueChange={m.setTab}>
        <TabsList variant="line" className="r-segmented-tabs">
          {[
            ["gallery", "媒体库"],
            ["identity", "视觉身份"],
            ["generate", "生成画面"],
            ["avatar", "显示头像"],
          ].map(([id, label]) => (
            <TabsTrigger key={id} value={id}>
              {label}
              {id === "gallery" && m.library.ready && (
                <span>
                  {m.library.items.length}
                  {m.library.cursor ? "+" : ""}
                </span>
              )}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="gallery">
          <form
            className="r-toolbar r-wrap media-toolbar"
            onSubmit={(event) => {
              event.preventDefault();
              m.setFilter({ ...m.filter, q: m.search.trim() });
            }}
          >
            <div className="r-search">
              <Button type="submit" variant="ghost" size="icon" aria-label="查询媒体"><Search size={15}/></Button>
              <FieldInput
                id="media-query"
                aria-label="搜索喜欢的画面"
                type="search"
                placeholder="搜索喜欢的画面"
                value={m.search}
                onChange={(event) => m.setSearch(event.target.value)}
              />
            </div>
            <ChoiceSelect
              aria-label="媒体类型"
              value={m.filter.kind}
              onChange={(event) =>
                m.setFilter({ ...m.filter, kind: event.target.value })
              }
            >
              <option value="">全部画面</option>
              <option value="image">图片</option>
              <option value="video">视频</option>
              <option value="audio">音频</option>
            </ChoiceSelect>
            <span className="r-fineprint">保存的素材 · 画面不改变生活状态</span>
            <Disclosure className="media-options" title="来源、角色与库中展示">
              <div className="media-filter-details">
                <ChoiceSelect
                  aria-label="媒体角色范围"
                  value={m.filter.fid}
                  onChange={(e) =>
                    m.setFilter({ ...m.filter, fid: e.target.value })
                  }
                >
                  <option value="">我的全部素材</option>
                  {m.filter.fid &&
                    !m.w.state.characters.some(
                      (c) => c.id === m.filter.fid,
                    ) && (
                      <option value={m.filter.fid}>
                        已指定角色范围（名称待读取）
                      </option>
                    )}
                  {m.w.state.characters.map((c) => (
                    <option value={c.id} key={c.id}>
                      {c.name}的关联素材
                    </option>
                  ))}
                </ChoiceSelect>
                <ChoiceSelect
                  aria-label="素材来源"
                  value={m.filter.origin}
                  onChange={(e) =>
                    m.setFilter({ ...m.filter, origin: e.target.value })
                  }
                >
                  <option value="all">全部来源</option>
                  <option value="owner_upload">我的上传</option>
                  <option value="studio">创作素材</option>
                  <option value="fluctlight">角色生成</option>
                </ChoiceSelect>
                <label className="companion-checkbox-row">
                  <Switch
                    aria-label="只看已隐藏素材"
                    checked={m.filter.hidden}
                    onCheckedChange={(hidden) =>
                      m.setFilter({ ...m.filter, hidden })
                    }
                  />
                  只看已隐藏
                </label>
              </div>
            </Disclosure>
          </form>

          <MediaReader source={m.library} label="素材" />
          <div className="r-media-grid">
            {m.library.items.map((asset) => (
              <Surface className="panel r-media-card" key={asset.id}>
                <button
                  className="r-media-image-button"
                  aria-label={
                    "查看画面 " +
                    (asset.title || asset.source_filename || "未命名素材")
                  }
                  onClick={(event) =>
                    m.openAsset(asset.id, event.currentTarget)
                  }
                >
                  <SavedMedia
                    asset={asset}
                    model={m}
                    thumbnail
                    className="media-card-art"
                  />
                  <span className="r-media-hover">
                    <Eye size={20} />
                    查看画面
                  </span>
                </button>
                <div className="r-card-body">
                  <span className="r-tiny-label">{mediaKind(asset.kind)}</span>
                  <h3>
                    {asset.title || asset.source_filename || "未命名素材"}
                  </h3>
                  <div className="r-card-footer">
                    <small title={asset.created_at?userInstant(asset.created_at):undefined}>
                      {asset.created_at
                        ? compactDateTime(asset.created_at)||userInstant(asset.created_at)
                        : "时间未记录"}
                    </small>
                    <small>{mediaSource(asset)}</small>
                  </div>
                </div>
              </Surface>
            ))}
          </div>
          {m.library.ready && !m.library.busy && !m.library.error && !m.library.items.length && (
            <EmptyState variant="page">
              <h3>还没有这样的画面</h3>
              <p>{m.w.state.characters.length?'上传或创作一张新的画面，慢慢收集生活的片刻。':'先创建一个角色，再把想象留在画面里。'}</p>
              <Button onClick={()=>m.w.state.characters.length?m.setTab('generate'):m.w.navigatePath('/library/create')}>开始创作</Button>
              <Button
                variant="outline"
                onClick={() => {
                  m.setSearch("");
                  m.setFilter({
                    q: "",
                    fid: "",
                    kind: "",
                    origin: "all",
                    hidden: false,
                  });
                }}
              >
                清除筛选
              </Button>
            </EmptyState>
          )}
          <MediaMore source={m.library} label="素材" />
        </TabsContent>
        <TabsContent value="identity">
          {m.target ? (
            <VisualWorkspace key={m.target} model={m} />
          ) : (
            <WorkspaceInfo>先选择一个角色，再查看其视觉身份。</WorkspaceInfo>
          )}
        </TabsContent>
        <TabsContent value="generate">
          {m.target ? (
            <MediaGeneration key={m.target} model={m} />
          ) : (
            <WorkspaceInfo>先选择一个角色，再提出图片请求。</WorkspaceInfo>
          )}
        </TabsContent>
        <TabsContent value="avatar">
          {m.target ? (
            <AvatarWorkspace key={m.target} model={m} />
          ) : (
            <WorkspaceInfo>先选择一个角色，再选择显示头像。</WorkspaceInfo>
          )}
        </TabsContent>
      </Tabs>
      {m.selected && (
        <MediaDetail
          key={m.selected}
          model={m}
          id={m.selected}
          onClose={m.closeAsset}
        />
      )}
      <WorkspaceDialog
        open={m.uploadOpen}
        onOpenChange={m.setUploadOpen}
        title="添加素材"
        description="保存到你的媒体库，保留上传来源"
        returnFocus={() => m.trigger.current}
      >
        <MediaUpload
          onSaved={async () => {
            await m.library.load();
          }}
        />
      </WorkspaceDialog>
    </div>
  );
}

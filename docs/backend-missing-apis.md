# 后端缺失接口清单

汇总阶段 1–2（外壳与聊天）、阶段 3（生活、关系、记忆、媒体）和阶段 4–5（创作、世界、动态、自治、数据工具、诊断剩余项）。只列浏览器所有者接口仍缺、因而页面必须显示「尚未接入」或「未提供」的能力。已有并被页面调用的接口不在此重复。

分类：

- **B 类补入口**：领域里已经有读模型、相邻路由或内部能力，缺的是所有者浏览器路由、响应字段，或页面还不能安全调用的那一步。
- **C 类新领域能力**：`browserOperations` 里没有这一领域的资源。新路由不能靠现有列表接口假装完成。

每项都要求所有者会话与 CSRF。写操作默认带 `Idempotency-Key`（或请求体 `idempotencyKey`）和期望版本；冲突返回 409，并带回当前 `revision`，前端保留草稿。原因和证据沿用现有治理字段：`reason`、`evidenceRefs`。浏览类 GET 不得移动角色或改写事实。

优先级：P1 挡住页面主操作；P2 补展示或次要操作；P3 本期明确不做，只保留说明页。

## B 类补入口

### B-01 此刻场景图

- 用途：此刻页英雄区显示当前场景，缺图时继续显示「场景图尚未接入」。
- 建议：扩展 `GET /api/fluctlights/{fluctlightId}/detail` 的 `current_state`。
- 响应字段：`scene_image` 或 `scene_url`（资产 id 或地址）、`captured_at`。没有键表示未接入；空字符串表示记录为空。
- 权限与版本：所有者只读。不要求幂等。不得把插画当成生活事实。
- 页面与 BP：`LifeNowView`，`life.now` / BP-06。
- 优先级：P1。

### B-02 心情与精力证据

- 用途：有心情标签时展示证据；没有 `evidence` 键时显示「证据字段未接入」，不标推测。
- 建议：同一 detail 的 `current_state.mood`、`current_state.momentum`。
- 响应字段：`label`、`evidence[]`（可为空数组）、`updated_at`。
- 权限与版本：所有者只读。
- 页面与 BP：`LifeNowView`，BP-06。
- 优先级：P1。

### B-03 逐条日程建议

- 用途：在已接受的整日日程上增删改一条，而不是每次提交完整 JSON。
- 建议：`POST /api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items`，`PATCH` 与 `DELETE` 同一资源。现有整日 `PUT /schedules` 保留。
- 请求字段：`activity`、`scene`、`location`、`startAt`、`endAt`、`evidenceRefs[]`、`expectedRevision`、`expectedLifeContextRevision`、`idempotencyKey`、`reason`。
- 响应字段（按契约 v0.1，Q3/Q4 待裁决）：`202 {request:OwnerRequest(kind:"schedule_item_suggestion"), schedule_revision, audit_id}`。日程要等 Agent `accepted` 后才变。409 的 `details.current_revision` 用于保留草稿。不在请求事务里改事实。
- 权限与版本：所有者写。幂等键与 revision 必填。
- 页面与 BP：`LifeScheduleView`，BP-06 / GOV-04。
- 优先级：P1。

### B-04 换装

- 用途：把已拥有或借用的物品标成正在穿着。穿着不改变所有权和来源。
- 建议：`POST /api/fluctlights/{fluctlightId}/wardrobe/wear`。内部已有 `wardrobe.wear` 能力，没有所有者浏览器路由。
- 请求字段：`itemId`、`slot`、`expectedRevision`、`idempotencyKey`、`reason`。管理视图二次确认后另带 `confirm: true`。
- 响应字段：交流视图 `202 {request:OwnerRequest(kind:"wardrobe_wear"), audit_id}`，接受前不改穿着。管理视图成功为 `200`，`status: "saved"`、`identity: "owner_edit"`、`actor: "owner"`、`audit_id`，并返回新的衣橱 `revision`。`expectedRevision` 传衣橱 GET 的 `revision`，不传 `owner_revision`。`owner_revision` 只展示所有者修改计数。409 为 `revision_conflict`，`details.resource` 为 `"wardrobe"`，`details.current_revision` 是她刚写成的衣橱版本；草稿保留，并提示她刚更新过。`worn_items[].recorded_by` 为 `owner` 时显示「所有者修改」，为 `fluctlight` 时显示她自己。
- 权限与版本：所有者写。幂等键必填。管理视图的期望版本是衣橱 `revision`。
- 页面与 BP：`LifeWardrobeView`，`items.wardrobe.view` / BP-08。
- 优先级：P1。

### B-05 开始或结束使用

- 用途：记录物品正在使用。使用、穿着、拥有、来源继续分列。
- 建议：`POST /api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}/use`，结束使用用同一路由加 `active: false`。
- 请求字段：`active`、`expectedRevision`、`idempotencyKey`、`reason`、`evidenceRefs[]`。管理视图二次确认后另带 `confirm: true`，`expectedRevision` 同样是衣橱 `revision`。
- 响应字段：交流视图 `202 {request:OwnerRequest(kind:"item_use"), audit_id}`。管理视图成功与换装相同：`status: "saved"`、`identity: "owner_edit"`、`actor: "owner"`、`audit_id`。409 同样是 `revision_conflict` 且 `details.resource` 为 `"wardrobe"`。`used_items[].recorded_by` 为 `owner` 或 `fluctlight`，显示「所有者修改」或她自己。
- 权限与版本：所有者写。幂等键与 revision 必填。
- 页面与 BP：衣柜与 `items.item.detail#usage`，GAP-10 / BP-08。
- 优先级：P1。

### B-06 入库中状态

- 用途：衣柜筛选「入库中」。现在没有该字段，筛选固定显示尚未接入。
- 建议：衣柜物品增加 `inbound_status`。
- 响应字段：`pending | failed | ready`、`failure_reason`、`source_action_id`。缺键表示未接入，不要返回空数组冒充。
- 权限与版本：所有者只读；状态由服务端行动写入。
- 页面与 BP：`LifeWardrobeView`，BP-08。
- 优先级：P2。

### B-07 物品拍照且不改事实

- 用途：给物品留一张图，不改所有权、穿着或生活事实。
- 建议：`POST /api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}/photos`。
- 请求字段：`prompt`、`outfit_snapshot_id`、`idempotencyKey`。
- 响应字段：`asset_id`、`factual: false`。页面继续标「生成图·非事实」。
- 权限与版本：所有者写。幂等键必填。不接受 revision 改写物品事实。
- 页面与 BP：BP-08、BP-14。
- 优先级：P2。

### B-08 角色媒体库列表

- 用途：媒体页列出该角色的生成图。现在只有视觉身份时间线和诊断提示词。
- 建议：`GET /api/fluctlights/{fluctlightId}/media?cursor=`。
- 响应字段：`items[]`（`asset_id`、`kind`、`created_at`、`outfit_snapshot_id`、`item_id`、`factual: false`）、`next_cursor`。
- 权限与版本：所有者只读。创作预览资产不得出现在此列表。
- 页面与 BP：`MediaView`，BP-14 / DET-02。
- 优先级：P1。

### B-09 聊天侧栏摘要

- 用途：对话侧栏显示位置、下一项日程、穿着和目标，缺字段保持空白。
- 建议：`GET /api/fluctlights/{fluctlightId}/sidebar`，或在 detail 上固定这些键。
- 响应字段：`location`、`next_schedule`、`worn_item_ids[]`、`goal_ids[]`、`memory_ids[]`。每个键缺失表示未接入，空值表示没有记录。
- 权限与版本：所有者只读。不要用初始人格填空。
- 页面与 BP：`chat.thread.view`，BP-02、BP-06。
- 优先级：P2。

### B-10 角色头像

- 用途：顶栏和角色库显示可选头像。没有地址时继续用首字。
- 建议：实例列表与 detail 增加 `avatar_asset_id`。
- 响应字段：`avatar_asset_id`、`factual: false`。
- 权限与版本：所有者只读。生成图标「生成图·非事实」。
- 页面与 BP：外壳与角色库，BP-01。
- 优先级：P2。

### B-11 用户发布动态

- 用途：动态页已有列表、评论、反应、隐藏。缺的是用户自己发布。所有者不能以角色身份代发。
- 建议：`POST /api/moments`。作者固定为当前所有者 Actor。
- 请求字段：`text`、`media_ids[]`、`idempotencyKey`。不接受 `author_fluctlight_id` 或代发标志。
- 响应字段：`id`、`author_actor_id`、`created_at`。媒体继续标非事实。
- 权限与版本：所有者写。幂等键必填。重复键返回同一条。
- 页面与 BP：`MomentsView`，`feed.post.compose` / GAP-07 / MOM-02。
- 优先级：P1。

### B-12 附件上传

- 用途：聊天发送已能携带 `attachmentRefs`，没有上传入口。
- 建议：`POST /api/media` 返回 `asset_id`，再把 id 放进现有 turn。
- 请求字段：文件字节、`mime`、`idempotencyKey`。
- 响应字段：`asset_id`、`kind`。
- 权限与版本：所有者写。幂等键必填。上传失败不产生消息。
- 页面与 BP：`chat.media`，CHAT-04 / GAP-06。
- 优先级：P2。

### B-13 新会话分段

- 用途：`POST /api/conversations` 已存在，界面没有「新分段」。新分段不能重置长期记忆、关系或生活事实。
- 建议：沿用现有路由，响应里标明 `segment_id`，并保证 fluctlight 长期状态 revision 不变。
- 请求字段：`title`、`participantActorIds`、`idempotencyKey`。
- 响应字段：`conversation.id`。不得返回已清空的记忆或关系。
- 权限与版本：所有者写。幂等键必填。
- 页面与 BP：`chat.thread.new`，GAP-09。
- 优先级：P2。

### B-14 移出分组

- 用途：`DELETE /api/actor-groups/{groupId}/members/{actorId}` 已存在。角色库能归入分组，没有移出入口。移出只改归属，回到未分组。
- 建议：页面调用现有 DELETE。若删除分组，组成员全部回到未分组，需要 `DELETE /api/actor-groups/{groupId}`。
- 请求字段：无正文，或 `Idempotency-Key`。
- 响应字段：204，或更新后的分组成员。
- 权限与版本：所有者写。退休角色不能拖入普通分组。
- 页面与 BP：`InstancesView`，INS-01 / GAP-08。
- 优先级：P2。

### B-15 永久伤害拦截计数

- 用途：诊断首页只展示被拦截次数和说明。不提供放行。
- 建议：`GET /api/diagnostics/harm-intercepts`。不要新增放行 POST。
- 响应字段：`count`、`window`、`latest_reason`。接口不可用时前端继续显示尚未接入，不显示 0。
- 权限与版本：所有者只读。
- 页面与 BP：`DiagnosticsView` 概览，`diag.home` / BP-17。
- 优先级：P1。

### B-16 诊断任务稳定深链

- 用途：模型运行和工作流已能按 `correlation_id` 跳转。缺稳定任务 id 页 `/diag/tasks/:taskId`。
- 建议：`GET /api/diagnostics/tasks/{taskId}`。
- 响应字段：`task_id`、`type`、`initiator`、`steps[]`（`status`、`idempotency_key`、`error_code`）、`correlation_id`、`fluctlight_id`。
- 权限与版本：所有者只读。重试继续走现有媒体提示词重试或工作流命令，并带确认。
- 页面与 BP：DIA-02 / DIA-03，BP-17。
- 优先级：P2。

### B-17 修改角色视觉定义

- 用途：视觉身份时间线可读。设置里的 `visual_identity_workflow` 是全局渲染配置，不是这个角色的定义。
- 建议：`PUT /api/fluctlights/{fluctlightId}/visual-identity`。
- 请求字段：`identity_snapshot`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`current_revision`、`status`。
- 权限与版本：所有者写。revision 与幂等键必填。不得顺便改全局 ComfyUI 工作流。
- 页面与 BP：`MediaView`，BP-14、BP-05。
- 优先级：P2。

## C 类新领域能力

### C-01 通知

- 用途：通知中心列出「发生了什么」，可已读。铃铛不显示虚构未读数。
- 建议：`GET /api/notifications?cursor=`，`POST /api/notifications/{id}/read`。
- 请求字段：已读只需 `idempotencyKey`。
- 响应字段：`id`、`title`、`body`、`read_at`、`created_at`、`link`。没有接口时不要返回空列表冒充已读完。
- 权限与版本：所有者读写自己的通知。
- 页面与 BP：`UnwiredView` 通知中心，`inbox.notice.list` / BP-03。
- 优先级：P1。

### C-02 此地附近

- 用途：此刻页显示同一地点的在场 Actor。关系名单不是在场名单。`PUT /presence` 只写自己的在场，不能当附近查询。
- 建议：`GET /api/fluctlights/{fluctlightId}/nearby`。
- 响应字段：`place_id`、`actors[]`（`actor_id`、`npc`、`since`）。缺接口时页面保持「此地附近尚未接入」。
- 权限与版本：所有者只读。GET 不得移动角色。
- 页面与 BP：`LifeNowView`，BP-06、BP-12。
- 优先级：P2。

### C-03 日记与共同经历

- 用途：日记页读写条目。试生成和创作预览不能写入。
- 建议：`GET/POST /api/fluctlights/{fluctlightId}/diary`，`PATCH /api/fluctlights/{fluctlightId}/diary/{entryId}`。
- 请求字段：`body`、`occurred_at`、`event_id`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`id`、`revision`、`source`。
- 权限与版本：所有者写。revision 与幂等键必填。
- 页面与 BP：生活日记，BP-09。
- 优先级：P1。

### C-04 反思详情

- 用途：记忆页打开一条反思。Developing Self 的回滚已经存在，不能代替反思正文，也不能改写 Core Persona。
- 建议：`GET /api/fluctlights/{fluctlightId}/reflections/{reflectionId}`。
- 响应字段：`id`、`body`、`responded_at`、`claim_ids[]`。
- 权限与版本：所有者只读。若以后允许回应，再单列带 `reason` 的 POST。
- 页面与 BP：`GrowthView`，BP-13.4 / GAP-01。
- 优先级：P2。

### C-05 世界地点、场景与地图

- 用途：世界页除本实例已记录事件外，还要地点、场景、地图和 NPC 图。浏览不得调用会改位置的接口。
- 建议：`GET /api/fluctlights/{fluctlightId}/world`。
- 响应字段：`places[]`、`scenes[]`、`map`、`npc_ids[]`。事件继续读 detail.`events`。
- 权限与版本：所有者只读。不提供「前往」写路由，除非另有带 revision 的行动接口。
- 页面与 BP：`WorldView`，BP-10、BP-11 / GAP-03。
- 优先级：P1。

### C-06 实例事件模板开关

- 用途：已发布的全局事件模板，由该实例开关。世界页不能创建模板。
- 建议：`PUT /api/fluctlights/{fluctlightId}/world/templates/{templateId}`。
- 请求字段：`enabled`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`template_id`、`enabled`、`revision`。
- 权限与版本：所有者写。含永久伤害的模板不能被开关绕过，服务端拒绝。
- 页面与 BP：`world.instance.settings`，BP-11。
- 优先级：P2。

### C-07 创作资源

- 用途：创作中心的角色模板、人格片段、地点设定、物品、事件模板、视觉风格。编辑定义不自动改变运行中的实例。
- 建议：`GET/POST /api/studio/resources`，`PUT /api/studio/resources/{resourceId}`。
- 请求字段：`kind`、`body`、`expectedVersion`、`idempotencyKey`、`reason`。
- 响应字段：`id`、`version`、`referenced_by[]`。版本冲突 409，并给出双方版本。含永久伤害事件的模板直接 422，导入和复制同样拒绝。
- 权限与版本：所有者写。版本号与幂等键必填。
- 页面与 BP：`StudioView`，`studio.resource.edit` / BP-15 / GAP-04。
- 优先级：P1。

### C-08 设定助手试生成

- 用途：把现在只留在浏览器里的预览会话存成创作资源。结果不得写入角色媒体库、日记、记忆或生活事实。
- 建议：`POST /api/studio/previews`。
- 请求字段：`resource_id`、`text`、`idempotencyKey`。
- 响应字段：`preview_id`、`text`、`label: "创作预览·不进入生活"`、`life_write: false`。
- 权限与版本：所有者写。幂等键必填。服务端拒绝任何附带的生活写入。
- 页面与 BP：`StudioView` 设定助手，`studio.preview` / BP-15、BP-14 / J-07。
- 优先级：P1。

### C-09 应用到实例

- 用途：把某个资源版本应用到一个角色，先看差异和冲突。
- 建议：`POST /api/studio/resources/{resourceId}/apply`。深链 `/studio/apply?resource=:rid&char=:charId` 已能打开说明，没有提交。
- 请求字段：`fluctlightId`、`resourceVersion`、`expectedInstanceRevision`、`choices[]`（保留或覆盖）、`idempotencyKey`、`reason`。
- 响应字段：`status: "recorded_not_applied"`、`applied: false`、`resource_version`、`instance_revision`（可为 null）、`choices`、`conflicts: null`。界面只标「未完成·已记录选择，尚未写入实例」。不读取已删除的应用版本字段，也不把 `conflicts: null` 显示成冲突已解决。
- 权限与版本：所有者写，记审计。版本、实例 revision、幂等键、原因都必填。未完成响应不写实例。
- 页面与 BP：`studio.apply.drawer`，BP-15、BP-05 / GOV-10。
- 优先级：P1。

### C-10 角色级自治与触达

- 用途：全局 `product.autonomy` 已能保存。角色级免打扰、花费上限、主动触达和需确认的行动类型还没有。
- 建议：`GET/PUT /api/fluctlights/{fluctlightId}/autonomy-policy`。
- 请求字段：`level`、`quiet_hours`、`spend_cap`、`outreach_limit`、`confirm_action_types[]`、`expectedRevision`、`idempotencyKey`、`reason`。
- 响应字段：`revision`、`effective_at`。
- 权限与版本：所有者写，记审计。不得用全局预算冒充角色上限。
- 页面与 BP：`AutonomyView`，`settings.char.autonomy#outreach` / BP-16 / GOV-03。
- 优先级：P1。

### C-11 待处理聚合

- 用途：铃铛打开的待处理页。能力审核列表已有，其余来源没有聚合。本接口只返回条目，不批准。
- 建议：`GET /api/pending?fluctlightId=`。
- 响应字段：`groups[]`，每组 `kind`（`capability`、`persona_proposal`、`studio_apply`、`failed_task`）、`status: "ok" | "unwired"`、`items[]`（`id`、`title`、`href`）。`unwired` 不能写成 `items: []`。
- 权限与版本：所有者只读。批准仍走原页面：能力审核、人格提案、创作应用。
- 页面与 BP：`PendingView`，`diag.pending.list` / BP-17.7、BP-16.6 / GOV-07。
- 优先级：P1。

### C-12 配置包导入

- 用途：导入全局定义，成为创作中心新版本。不是读档，不含运行事实。JSON 创建角色走现有创建分析，不属于恢复。
- 建议：`GET /api/studio/packs/export`，`POST /api/studio/packs/validate`，`POST /api/studio/packs`。
- 请求字段：包内容、`idempotencyKey`。
- 响应字段：导出 `{pack, available}`；校验 `{valid, status:"validated"}`；导入 `{pack_id, resource_versions, status:"draft"}`。永久伤害模板返回 422。导入只生成草稿，不发布、不应用到实例。
- 权限与版本：所有者写。幂等键必填。之后必须走 C-09。
- 页面与 BP：`DataToolsView` 配置包栏，BP-18 / INS-02。
- 优先级：P2。

### C-13 业务数据备份与恢复

- 用途：灾备和换机。不提供历史时间点选择，不能当剧情试错。
- 建议：`POST /api/ops/backups`，`POST /api/ops/backups/{backupId}/restore`。
- 请求字段：恢复时 `confirm: true`、`idempotencyKey`、`reason`。
- 响应字段：未接入时 `{available:false, status:"not_implemented"}`。恢复按钮在 `available:false` 时禁用，不显示成 0 份备份。
- 权限与版本：所有者写，记审计。恢复必须二次确认。
- 页面与 BP：`DataToolsView` 备份栏，BP-18。
- 优先级：P2。

### C-14 时间线回档

- 用途：检查点恢复。本期不实现。说明页标记「未提供」，并写明对日记、记忆、关系的影响。
- 建议：将来才考虑 `POST /api/ops/timeline-checkpoints/{checkpointId}/restore`。现在不要提供该路由。
- 请求字段（将来）：`checkpointId`、`confirm`、`idempotencyKey`、`reason`，以及受影响的日记、记忆、关系 revision。
- 响应字段（将来）：`restored_at`、`audit_id`。没有路由时前端不显示 0 个检查点。
- 权限与版本：若实现，所有者写，二次确认，记审计。
- 页面与 BP：`RollbackView`，`data.rollback.view` / BP-18 / J-09。
- 优先级：P3。本期保持未提供。

### C-15 人格切换

- 用途：在角色资料里切换人格，和基础修订不是同一步。
- 建议：`POST /api/fluctlights/{fluctlightId}/persona-switch`。
- 请求字段：`targetPersonaId`、`expectedRevision`、`idempotencyKey`、`reason`、`confirm: true`。
- 响应字段：`revision`、`audit_id`、`status: "pending"`。没有可选人格列表 GET，缺列表时不编造人格。
- 权限与版本：所有者写。二次确认。不得把切换直接写成运行状态。
- 页面与 BP：`profile.persona.switch`，GOV-12。
- 优先级：P2。

### C-16 Agent 与 Tool 定义只读

- 用途：设置里查看 Agent / Tool 定义。P3 只读，页面不能保存。
- 建议：`GET /api/settings/agents`。
- 响应字段：`agents[]`、`tools[]`、`version`。无 PUT。
- 权限与版本：所有者只读。
- 页面与 BP：`settings.models#agents`，GAP-02。
- 优先级：P3。

### C-17 独立 Desire 列表

- 用途：与 Goal 分开的愿望列表。目标规划接口不能冒充。
- 建议：`GET /api/fluctlights/{fluctlightId}/desires`。
- 响应字段：`items[]`（`id`、`text`、`status`）。
- 权限与版本：先只读。写操作若增加，必须带 revision 和原因。
- 页面与 BP：`goals.desire.list`，GAP-05。
- 优先级：P3。

## X 类：契约补充（v0.1，清单外）

前端 `BrowserClient` 已按契约实现这些读接口。响应 snake_case。写请求体使用 `idempotencyKey` 与 `expectedRevision`（Q1 待裁决：CORS 尚未放行 `Idempotency-Key` 头；multipart B-12 例外，键在请求头）。409 读取 `details.current_revision` 并保留草稿。缺键显示尚未接入，`null` 是无记录，`[]` 是空。

| 编号 | 路由 | 页面用途 |
|---|---|---|
| X-01 | `GET /api/notifications/unread-count` | 顶栏红点。404 时不显示假计数 |
| X-02 | `GET /api/pending/count` | 待处理计数。`partial` 表示有未接入分组 |
| X-03 | 列表上的 `now_line`、`status`、`status_note`、`group_id` | 角色库此刻一句话。另需 `bio` 才能还原卡片副标题（契约未单列，缺键则不编造简介） |
| X-04 | 消息 `identity` / `kind` / `recall` / `memory_ref` / `goal_ref` / `action_id` / `link` | 交流/管理只影响视图，不作为服务器授权 |
| X-05 | `POST /api/memories/{id}/suggestions` → 202 `memory_suggestion` | “告诉她”。不直接改记忆 |
| X-06 | `GET /api/owner-requests/{id}` 与按角色列表 | 处理中 / 失败。状态只由 Agent 推进 |
| X-07 | `POST /api/fluctlights/{fid}/promises` → 202 | 约定提议 |
| X-08 | `POST .../moment-suggestions` → 202 | 角色发帖建议。没有代发 |
| X-09 | `GET .../world/places/{placeId}` | 地点详情。纯 GET |
| X-10 | `GET .../world/templates` | 实例模板与永久伤害锁定 |
| X-11 | `POST .../apply-preview` | 只读差异，不需要幂等键 |
| X-12 | detail.events[].impacts | 事件影响链 |
| X-13 | 记忆 / 关系 `revisions[]` | 修订历史 |
| X-14 | 动态 `author_type`、`author_name`、`author_avatar_url`、`source_action_id` | 作者渲染 |

诊断首页还要从 detail 读到当前 `actions`（契约没有单独的行动列表，待裁决）。反思时间线需要 `GET .../reflections` 列表；契约 C-04 只有按 id 读取，客户端方法已标注待裁决，404 时不编造反思。

### 契约 §6 存疑点（先按当前写法）

- Q1：JSON 写操作使用请求体 `idempotencyKey`。日程条目更新与日记更新按已合入的 OpenAPI 使用 PUT。上传同时带请求头 `Idempotency-Key` 和表单字段 `idempotencyKey`。
- Q2：新响应使用 snake_case。
- Q3 / Q4：交流视图的换装、使用、逐条日程是 202 请求，不把本地结果写成事实。管理视图换装和使用在二次确认后带 `confirm: true`，成功回执是 `status: "saved"`、`identity: "owner_edit"`、`actor: "owner"`。衣橱的 `expectedRevision` 是衣橱 `revision`。409 的 `details.resource` 是 `wardrobe`。
- Q5：人工插画头像 `source: "owner_upload"`、`kind: "visual_identity"`，不打「生成图·非事实」。其余生成图仍打该标。
- Q6：反思回应是 `POST .../reflections/{id}/feedback`，只提交候选反馈，不改反思正文。
- Q7：配置包有导出、校验、导入。导入只生成草稿。
- Q8：没有人格列表 GET。切换前的列表缺键则显示尚未接入，不编造人格。
- 通知未读数和标记已读未接入时是 404 `not_provided`。红点不显示，不把缺计数当成 0。
- 分组删除的 `expectedRevision` 固定为 0。
- 创作应用的未完成状态只显示「未完成·已记录选择，尚未写入实例」。
- Q9–Q12：备份在 `available:false` 时禁用恢复；C-10 响应必须带 `runtime_effect: "preference_only"`。花费上限和 identity 中文对照仍按当前契约。

## 本期明确不接成可用功能

- 桌面宠物与 Live2D。
- 时间线回档的恢复按钮（C-14）。
- 永久伤害放行（B-15 只有计数）。
- 创作预览写入生活（C-08 必须拒绝）。
- 所有者代角色发动态（B-11 必须拒绝）。
- 浏览世界时移动角色（C-05 的 GET 不得写位置）。

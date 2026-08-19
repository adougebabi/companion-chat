# 人格媒体意图、模板化与结果验收：实施计划

## 前置约束

- 不开始实现前先加载 `trellis-before-dev` 的 backend 与 frontend 层指南。
- 修改前全仓搜索每个共享字段、job 类型、消息 generation 状态和 debug 字段的生产者/消费者。
- 改动集中在既有 `server.js`、`test/`、活跃 Companion UI；不增加数据库、队列、浏览器 provider 调用或新的视觉模型设置。

## 实施顺序

1. **定义并测试 A 的能力调用 schema。**
   - 将现有媒体 marker / 活动 decision contract 改为 `MediaCapabilityCallV2`，加入 `personaMediaConcept`、`currentEvent`、`temporaryAppearance`。
   - 复用 `normalizePersonaMediaConcept()`；新增调用对象 normalizer，确保 kind/concept mediaKind、count、边界长度和对象形状有效。
   - 将服务器构造的 event/state/appearance 设为权威；模型字段只保留作可审计调用输入，不能覆盖事实。
   - 更新聊天提示与活动 decision 提示，确保活动模型收到 event appearance。

2. **贯通三类生产者并冻结 job payload。**
   - 更新 chat extraction、activity decision parsing、`createChatMediaRequest()`、活动入队和 debug route，使它们保留同一规范化调用对象。
   - job 创建时持久化 `{ envelope, personaMediaConcept, capabilityCall, kind, provider, trigger, qualityRetryCount, maxQualityRetries }`。
   - 保留一次 count 对应一个 job/占位的现有行为；每个 job 复制同一冻结事实和概念。

3. **把 worker 从 A 阶段移除。**
   - `submitMediaJob()` 只读取/验证 payload 中的 frozen A；不再在正常路径调用 `generatePersonaMediaConcept()`。
   - 旧 payload 缺概念时，终止媒体目标并保存 `missing_frozen_media_concept` 诊断；不得调用 B 或 provider。
   - `result_json.personaConcept` 保持兼容，并标注来源；扩展 debug summary 使 queued job 也能看到冻结概念。

4. **让 B 支持一次由 C 驱动的重试。**
   - 给 B 增加可选、受限的 prior-acceptance violation input；仅允许强调冻结事实，不允许改写 A。
   - 新增建立继任 source job 的专用 helper：同一 target/provider/facts/concept，`qualityRetryCount` 从 0 增至 1，不产生第二个占位项。
   - 区分 provider/传输 retry 与 quality retry，避免复用 poll job 的 60 次上限。

5. **实现候选媒体的服务端读取与 C 调用。**
   - 为各 provider 增加受限的内部候选字节读取边界；不调用公开 Express asset route。
   - 图片生成 bounded data URL 多模态内容；使用当前 `lmCompletion()` 的模型设置。
   - 为 C 新建严格 JSON contract / normalizer，限制 `pass|retry|reject`、违例条数/长度和 retry guidance。
   - 将模型/字节/帧/解析不可用明确归类为 `skipped`，不与模型内容 verdict 混淆。

6. **实现有界视频关键帧验收。**
   - 使用固定参数数组调用 `ffprobe` / `ffmpeg`，在临时受控目录提取少量 JPEG 帧。
   - 限制视频路径、时长探测超时、抽帧数、缩放像素、单帧/总字节和 stderr；清理临时文件。
   - 无法安全提取帧时以 `skipped` 交付，不阻塞已成功 provider 结果。

7. **将 C 插入最终交付前的状态转换。**
   - 同步 submit 与异步 poll 成功路径都在创建 `companion_media_assets`、更新 target `ready` 前执行 C。
   - `pass` / `skipped` 才持久化并关联资产。
   - 第一次 `retry` 记录验收、入队继任 source job；第二次 retry 或任意 `reject` 不关联候选资产、令目标失败但正文保留。
   - 重新审计 lease 校验和幂等完成，确保 stale/double completion 不能创建资产或二次重试。

8. **更新 UI、诊断和文档。**
   - 复用现有 failed media 卡片与 generation/activity status，给旧 job/质量拒绝提供不泄露内部细节的重新生成说明。
   - 扩展 debug inspector 的 persona-scoped、bounded projection；不显示 credentials、路径、data URL 或完整命令。
   - 将最终 A/B/C authority、C fail-open 条件与旧 job 迁移规则同步到 backend media prompt / debug observability specs（必要时通过 `trellis-update-spec`）。

## 验证计划

### 单元与回归

- `npm test`
- `node --check server.js`
- `node --check src/companion-main.js`
- `git diff --check`

新增/更新 fixture 至少覆盖：

1. 聊天、活动和调试三条入口都持久化相同形状的 A/事实快照。
2. 一个自拍、一个外部他拍/摄影者 POV、一个含衣物/宠物/屏幕等 non-human object 的概念，由模型 fixture 而非服务端规则决定。
3. 临时外观进入调用对象、envelope、B 输入与 C 验收。
4. 新 worker 绝不调用 `generatePersonaMediaConcept()`；旧 job 缺 frozen A 时无 B/provider 调用且仅失败媒体目标。
5. B 的八段渲染固定，C retry 只能携带冻结事实的违例。
6. image `pass` 在 C 后才关联 asset；`reject` 不关联；第一次 `retry` 仅产生一个继任 job；第二次不合格终止。
7. 图片与视频的 C `skipped`（视觉模型失败、超时、不支持、非法 JSON、无法安全读字节/帧）都会安全交付并有 redacted diagnostic。
8. 视频关键帧数量、像素/字节、临时目录和超时上限；ComfyUI 与 h3 候选都能通过内部读取边界进入 C。
9. stale lease、重复 poll 和重复完成不能绕过 C 或重复交付。
10. debug 输出包含 capability call / frozen concept / template / final prompt / acceptance，且保持 persona-scoped 与脱敏。

### 手工集成检查

- 用临时 `DATA_DIR` 启动服务并检查 `GET /api/health`；不触碰已检出的 `data/`。
- 通过实际视觉模型与图片 provider 完成一条通过 C 的聊天媒体请求；确认刷新后仍为 `ready`。
- 用可控 fixture/模拟 provider 验证一次质量 retry、终止 reject 与 C 不可用时的 skipped 交付。
- 用视频 provider 验证关键帧抽取的资源上限、超时降级和普通 UI 中的视频播放。
- 在桌面与窄屏中检查 queued/processing/ready/failed 文案、刷新恢复和无浏览器 console error。

## 高风险点与回滚点

| 风险 | 防护 / 回滚 |
| --- | --- |
| 模型输出的调用参数不完整 | 创建 job 前 schema 拒绝；不依赖 server 语义 fallback。 |
| 旧 job 在部署后悄悄重建概念 | 缺 frozen A 即 terminal fail；回归测试断言无 provider 调用。 |
| C 使 provider 成功结果无法交付 | 只有基础设施不可用才 `skipped` 交付；明确内容 verdict 有一次受限 retry。 |
| 视频验收耗尽资源或执行不安全命令 | 固定参数数组、临时目录、路径/帧/像素/字节/时间限制；无法验证则 skipped。 |
| poll / lease 双重完成 | 在每个 C 后的 asset/target 写入沿用匹配 lease 的完成事务；新增 stale/duplicate regression。 |
| 回滚代码后 JSON payload 不兼容 | payload/result 使用加法字段；老 worker 行为不作为兼容 fallback。 |

## 开始实施前的检查清单

- [ ] 用户审阅并批准 `prd.md`、`design.md`、`implement.md`。
- [ ] `implement.jsonl` / `check.jsonl` 含真实、相关的规范条目。
- [ ] 加载 Phase 1.3/1.4 的 Trellis 任务配置与 review gate。
- [ ] 确认未将用户已有的其他未提交改动纳入本任务。


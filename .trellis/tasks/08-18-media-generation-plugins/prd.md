# 插件化图片与视频生成接入

## Goal

将图片/视频生成从当前写死的 ComfyUI 流程改造成服务端可选择的媒体生成插件体系，使同一套媒体意图、提示词编译、持久化任务和前端消息占位逻辑可以接入不同实现；首个目标插件是用户本地通过命令行运行的 `h3.c` 视频生成程序。

## User Value

- 用户可以按媒体类型选择不同生成插件，而不必在本项目内再封装一层 ComfyUI 工作流。
- 现有 ComfyUI 使用方式可以继续工作，避免已有图片/视频任务和配置失效。
- 本地 `h3` 命令可以作为视频生成后端接入，例如使用模型目录、提示词、尺寸、帧数、步数等参数生成 MP4。

## Confirmed Facts

- `server.js` 是单一 Node 22 ES module，媒体任务持久化在 SQLite `companion_jobs`，通过 lease 由单 worker 执行。
- `companion_messages.generation_json`、`attachments_json` 和活动媒体表承担媒体状态与原位替换；这些用户体验不应因插件化改变。
- 当前 `submitMediaJob()`、`pollMedia()`、媒体资源代理和 `media_assets.provider` 全部硬编码为 ComfyUI。
- 当前设置只有 `comfyUrl`、`imageWorkflow`、`videoWorkflow`，没有 provider/plugin registry。
- 媒体意图与提示词编译已有严格的应用层契约；插件只能接收最终确定的媒体请求，不能改变人物、事件、动作和安全约束。
- 当前媒体完成没有独立 SSE，前端依靠 bootstrap/刷新读取原位更新；本任务不默认扩大为实时进度系统。
- 仓库规范禁止浏览器直连 provider，要求 provider 调用在事务外、租约条件下结算，并保留敏感信息脱敏。

## Requirements

### R1. Provider/plugin contract

定义服务端媒体插件接口，至少覆盖：提交任务、查询任务状态/输出、读取生成资产、能力声明（image/video）。插件返回 provider 名称和外部任务标识；队列、租约、重试、消息/活动状态由现有系统统一管理。

### R2. Built-in compatibility

将现有 ComfyUI 行为收敛到默认插件适配器，兼容当前 URL、图片/视频 workflow JSON、`{{prompt}}` 注入、history 轮询、`/view` 资源代理、资产去重和错误语义。已有配置不迁移时，行为保持可诊断的失败而不是静默改变。

### R3. h3.c video plugin

提供一个服务端可配置的本地 `h3.c` 命令插件，能将媒体意图映射到受控的命令参数并等待/轮询输出 MP4，至少支持用户示例中的模型目录、prompt、width、height、frames、steps、layers、reuse、SSD streaming 和输出路径等配置。插件不得让浏览器直接执行命令，也不得通过未校验的整段 shell 字符串执行。

### R4. Plugin selection and settings

设置 API 与当前设置界面能够展示已注册插件、其能力和每种媒体类型的默认选择；配置保存应校验插件存在且支持对应 kind。旧的 ComfyUI 设置字段在兼容期内保留或可明确迁移。

### R5. Cross-layer state and diagnostics

任务 payload/result、debug inspector 和媒体资产 locator 增加 provider/external job 信息，但普通 API 不暴露工作流、命令行参数中的敏感值或完整调试内容。失败、重试、取消/进程异常（若插件支持）均更新现有 generation/activity 状态。

### R6. Tests and documentation

新增 fake plugin 测试覆盖提交、轮询、资产读取、重试和 provider 选择；保留现有媒体意图/ComfyUI 兼容测试；补充 README 的插件配置和 h3 命令示例。

## Acceptance Criteria

- [x] 在不改变现有 ComfyUI 配置的情况下，图片和视频任务仍可排队、完成、失败并原位更新消息/活动。
- [x] 通过设置选择 h3 插件后，视频任务不访问 ComfyUI，而是以参数数组启动配置的本地 `h3` 可执行文件，并能将生成的 MP4 注册为媒体资产并通过现有媒体 URL 读取。
- [x] 未配置或不支持目标 kind 的插件会在任务层返回明确错误，并遵守现有重试/失败状态，不产生孤儿任务。
- [x] 插件进程退出码、超时、无输出文件和非法外部任务 ID 都有可测试且不泄露命令敏感值的错误路径。
- [x] 浏览器不直接调用 ComfyUI、h3 或任意插件；设置和 debug inspector 能显示当前 provider 与安全的状态摘要。
- [x] `npm test`、`node --check server.js` 以及临时 `DATA_DIR` 下的健康检查和代表性媒体请求通过；受限执行环境会在命令返回时回收临时监听进程，因此跨命令 HTTP 连通性由本地运行时复核。

## Out Of Scope

- 本任务不重写媒体意图、生活状态解析、提示词 refiner 或人物安全约束。
- 本任务不引入第二数据库、外部队列、独立服务进程或浏览器端插件运行时。
- 本任务不承诺实时视频进度 SSE、复杂工作流编辑器、远程插件市场或任意第三方插件沙箱。
- 本任务不默认删除 ComfyUI；兼容适配器会保留，后续可单独评估移除。

## Resolved Decision

首期采用服务端注册的受控本地命令插件。插件配置只允许可执行文件路径、模型/输出目录和结构化白名单参数，运行时使用 `spawn` 参数数组，不执行整段 shell 字符串；实现进程超时、退出码、输出文件和路径校验。这样可以直接覆盖本地 `h3.c`，代价是增加本地进程生命周期和权限边界处理。

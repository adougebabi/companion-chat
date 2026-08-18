# Technical Design: 媒体生成插件

## Boundaries

继续以 `server.js` 为唯一后端入口，在其中增加一个小型 provider registry 和两个适配器：`comfyui` 负责现有 HTTP workflow/history/view 协议，`h3` 负责受控本地进程。队列、SQLite lease、媒体意图编译、消息占位和资产表仍由现有 owner 管理；插件只拥有 provider-specific submit/poll/read 行为。

## Provider Contract

每个插件声明 `{id, label, capabilities: ['image'|'video']}`，并实现：

- `submit({kind, prompt, payload, settings}) -> {externalId, pending?, result?}`
- `poll({kind, externalId, settings}) -> {status: 'pending'|'complete'|'failed', files?, error?}`
- `readAsset({kind, locator, res, settings})`，由服务端媒体路由代理。

插件返回的 `externalId`、`provider` 和 `locator` 进入 job result / media asset locator；不得把 provider-specific 状态泄漏到普通聊天文本。

## Queue Data Flow

`createChatMediaRequest()` 继续创建 `chat_image`/`chat_video` durable job。worker 根据保存的 provider 选择适配器，调用 `submit`；完成提交后保存 `{provider, externalId}` 并创建统一的 `*_media_poll` job。poll worker 调用同一 provider 的 `poll`，完成后复用 `mediaAssets()` 和 `completePolledMediaJob()` 的原位更新路径。所有外部调用在事务外，settle 仍要求匹配 lease owner 和未过期 lease。

## ComfyUI Compatibility

默认选择保持 `comfyui`。适配器保留 URL 清理、workflow JSON 深拷贝、`{{prompt}}` 替换、`/prompt`、`/history/:id`、`/view` 和现有输出文件解析。旧 `comfyUrl/imageWorkflow/videoWorkflow` 字段继续有效；新 provider 选择字段只在明确选择其它插件时生效。

## h3 Command Plugin

插件使用 `spawn(executable, args, {stdio: ...})`，参数由结构化设置和已验证媒体 payload 生成。首期命令参数覆盖 `-d/--profile`、`-p`、`--width`、`--height`、`--frames`、`--steps`、`--layers`、`--reuse`、`--ssd-streaming`、`-o`。输出目录必须位于配置允许的根目录内；完成判定为进程退出码为 0 且输出文件存在、类型为 mp4、大小大于 0。命令插件可将一次进程视为同步任务，返回 deterministic external id，并在 poll 阶段读取已生成文件。

## Settings/API/UI

在 settings payload 增加 `mediaProviders`（注册表安全摘要）和 `imageProvider`/`videoProvider` 选择字段；保存时只接受已注册且支持 kind 的 ID。敏感命令路径、模型路径和 API key 不进入公开 bootstrap，debug 只显示 provider/id/status。当前设置界面增加按 kind 的 provider selector 和 h3 结构化配置字段，保留 ComfyUI workflow 字段。

## Compatibility, Failure, Rollback

旧任务 payload 没有 provider 时按 `comfyui` 解释。插件不存在、能力不匹配、进程超时、非零退出、缺少输出和非法 locator 都走现有 retry/failed 状态并更新 generation/activity。若新 provider 配置导致回归，可将 provider 选择切回 `comfyui`，无需迁移或删除历史资产；资产读取按每条资产记录的 provider 分派。

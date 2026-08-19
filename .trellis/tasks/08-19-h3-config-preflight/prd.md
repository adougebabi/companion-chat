# h3 配置回显与运行预检

## Goal

让本地 h3 视频生成配置可确认、可验证：设置页能安全回显当前运行服务保存的 h3 配置摘要；保存时能尽早拒绝无效路径；开发检查器可以不创建视频任务就测试 h3 是否能从当前运行环境启动。

## Confirmed Facts

- 仓库 `data/companion.sqlite` 已保存 `videoProvider: 'h3'`、绝对 h3 可执行文件路径、模型目录和输出目录；可执行文件存在且有执行权限。
- 旧视频任务报错 `spawn h3.c ENOENT`，它们均创建于正确绝对路径配置保存之前；保存后该数据库没有新的 video job，因此尚未验证新配置的运行时效果。
- 当前 `publicSettings()` 有意移除 `h3Executable/h3ModelDir/h3OutputDir/h3AllowedRoot`，导致设置页字段不会回显，用户无法确认正在运行服务的实际配置。
- Docker Compose 使用 `/app/data` volume，而当前 h3 是 macOS Mach-O arm64 可执行文件；若网页服务运行于 Linux 容器，它不能直接执行该 macOS 二进制。

## Requirements

### R1. Safe configuration summary

Bootstrap/settings API 返回 h3 的安全摘要，不返回完整绝对路径。摘要包含每项是否配置、末段可辨识名称（如 `…/h3`、`…/MiniMax-H3`、`…/outputs`）和校验状态；设置页据此显示已保存的当前配置而不是空字段。

### R2. Save-time validation

当保存 h3 配置或选择 h3 视频 provider 时，服务端验证：可执行文件为绝对路径、存在且可执行；模型目录存在且为目录；输出目录位于允许根内、可创建且可写。校验失败返回明确且不泄露不必要路径的 400 错误，不把无效配置写入 SQLite。

### R3. Runtime preflight

仅在 `COMPANION_DEBUG_INSPECTOR=1` 下注册 h3 预检 API。它使用当前运行服务的实际 settings，先做文件系统检查，再以短超时、无 shell 的 `h3 --help` 进程启动探测；不发送 prompt、不加载模型、不创建 durable media job、不写媒体资产。返回安全的检查摘要和受限输出/错误。

### R4. Debug UI

开发检查器提供“测试当前 h3 配置”入口，显示检查中、成功或失败摘要；设置页展示摘要和“已配置、路径不回显”的说明。失败信息应直接指出是 executable、模型目录、输出目录、进程启动还是环境类型问题。

### R5. Compatibility and tests

ComfyUI 和无 h3 配置的现有使用方式不受影响。测试覆盖安全摘要、相对路径/无执行权限/缺失目录拒绝、输出目录可写检查、预检不创建 job、debug route gate 和不泄露完整路径。

## Acceptance Criteria

- [x] 设置页能说明当前服务是否已配置 h3，并以安全摘要回显 executable、模型和输出目录，不显示空字段造成“配置丢失”的误解。
- [x] 保存 h3 配置时，无效/相对 executable、缺失模型目录或不可写输出目录会立刻返回可读错误且不持久化。
- [x] 开发检查器可不创建视频任务就检查当前运行服务的 h3 配置和 `--help` 启动能力。
- [x] 预检不执行生成命令、不会创建 `chat_video`/`activity_video` job 或媒体资产。
- [x] 完整路径、命令参数和敏感值不会进入普通 bootstrap、聊天消息或普通媒体 API。
- [x] `npm test`、脚本语法检查和 debug route gate 测试通过。

## Out Of Scope

- 不提供 Docker 内运行 macOS Mach-O h3 的兼容层；Docker 需要单独准备 Linux h3 二进制或 host-side provider。
- 不自动重新执行旧的 failed video job。
- 不把完整绝对路径公开到普通 settings bootstrap 响应。

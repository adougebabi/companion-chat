# Implementation Plan

1. [x] 阅读 backend/frontend/debug 规范与现有媒体任务 producer/consumer，确认不覆盖当前未提交的 Telegram/时间线前端变更。
2. [x] 在 `server.js` 增加进度类型、ANSI/路径安全清理、显式百分比解析、lease-guarded `recordMediaJobProgress()` 与有界节流 reporter。
3. [x] 改造 h3 `runH3()` 以捕获 stdout/stderr，按 `preparing → generating → validating_output → complete/failed` 写入 progress；保留 `shell:false` 和既有 h3 超时/输出文件校验。
4. [x] 在 `submitMediaJob()` 持久化 final prompt，并修改 completion/settle 路径合并既有 `result_json.progress`，确保同步 h3 与 ComfyUI polling 都不丢失安全终态。
5. [x] 扩展 `debugContextFor()` 的媒体 job DTO，加入稳定 progress fallback、实时 elapsed 投影、最终 prompt 以及脱敏/长度限制。
6. [x] 重新组织 `openInspector()` 的媒体区域为可读卡片，默认突出最终 prompt 和执行状态，次级折叠原始 intent/workflow；增加打开期间的 scoped polling 与关闭清理。
7. [x] 添加 h3 progress parser/writer/lease/terminal merge/debug DTO 测试，执行 `npm test`、`node --check server.js`、`node --check src/companion-main.js`、`git diff --check`；本地子进程流已验证，内置浏览器无法连接隔离的本机监听端口，因此未完成实时截图验证。

## Review Gates

- 进度 writer 使用与 settle 相同的 lease 条件，且 progress 不会隐式续租。
- 没有明确百分号时 UI 显示未知而非伪进度。
- 最终 prompt 只从编译后的 provider 输入读取；不把原始 intent 当作等价 prompt。
- stdout/stderr、路径、命令参数、token 和完整历史均不泄漏到公开 API。
- inspector polling 不会重建或干扰现有 dialog 表单；关闭时没有残留 interval。

# Implementation Plan

1. [x] 读取 backend/frontend 规范，建立 provider registry、统一 provider 配置规范和新增 migration/default settings。
2. [x] 抽取 ComfyUI adapter，替换 submit/poll/media asset route 中的硬编码调用，保留旧字段兼容。
3. [x] 实现受控 `h3` command adapter：结构化参数、spawn 参数数组、超时/退出/输出校验、MP4 locator。
4. [x] 将 provider/external id 写入 job payload/result、media asset locator 和 debug safe DTO；补齐 provider/capability 选择 API 校验。
5. [x] 更新当前 Companion 设置界面，增加 image/video provider 选择和 h3 配置；不让浏览器直连任何 provider。
6. [x] 增加 fake provider、ComfyUI compatibility、h3 command validation、失败重试、资产读取和设置校验测试；更新 README 示例。
7. [x] 运行 `npm test`、`node --check server.js`，使用临时 `DATA_DIR` 启动服务并验证启动路径；当前受限执行环境回收监听进程，未能从独立命令完成 `/api/health` HTTP 请求。

## Review Gates

- 任何新设置/迁移字段已搜索全部 producer/consumer。
- provider 调用不在 SQLite transaction 内，settle 有 lease guard。
- 不记录完整命令参数、API key、workflow 或 bearer 信息。
- 旧 ComfyUI 任务和无 provider 的历史任务可继续解释。

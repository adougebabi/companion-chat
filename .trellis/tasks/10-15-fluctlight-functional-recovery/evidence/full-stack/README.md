# 交付证据与本地产物

当前状态以 `concentrated-nonllm/progress.json` 和最终摘要为准。早期切片记录描述各自执行时的状态，不代表最终验收结果。真实模型统一验收仍由用户执行，17 个回归入口未执行 `--live`。

仓库保留结构化结果、公开备份/恢复回执、视觉对照截图及冻结 UI 基线。最终原始测试证据保留以下内容，便于对账：

- `core-final-shard-1.jsonl` 至 `core-final-shard-4.jsonl`：与 `core-final-manifest.json` 对应的完整 Core 注册集执行记录。
- `other-packages-final.jsonl`、`postgres-migration-verified.jsonl`：其余 Go 包和完整迁移组合。
- `v3-operations-postgres.jsonl`、`scheduled-resolution-recovery.jsonl`：数据工具及异步恢复的专项执行证据。
- 最终 Web、BrowserClient、工作区类型/测试/构建日志及 MinIO 官方源码镜像构建结果。

以上日志均位于 `concentrated-nonllm/`。本目录 `.gitignore` 精确保留这些最终记录；重复构建输出、修复前失败运行和中间定向重跑日志仅留在本地。执行历史中的这类路径用于追溯本地运行，不能视为仓库下载后必定存在的附件。完整功能台账和最终摘要不依赖这些中间日志作通过结论。

私有部署环境文件、账号密码、会话 Cookie、维护执行令牌、数据库及对象存储卷备份均位于仓库外的私有临时目录，不属于公开交付证据。

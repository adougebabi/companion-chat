# 实施前基线（2026-09-24）

## 代码与环境

- 基线提交：`0dc8055 fix(prompt): constrain media prompt output to eliminate preamble and reasoning`。任务目录创建后 `git status --short --branch` 仅出现未跟踪任务目录，产品代码无当前未提交改动。会话最初曾短暂显示三个修改文件，之后已消失；任何再出现的外部修改均逐段核对。
- Go Core 为生产领域与模型运行时；当前迁移头 `0036_effective_life`。现有 API/Worker 没有运行。未设置 `GO_CORE_TEST_DATABASE_URL`、`CORE_GO_DATABASE_URL`、`FLUCTLIGHT_SETTINGS_KEY` 或 `FLUCTLIGHT_LIVE_PROVIDER_*`；本轮未接触生产数据。
- 本机 `127.0.0.1:11234/v1/models` 返回 HTTP 200，声明一个已加载的可聊天、工具、结构化响应模型；尚未证明它与 Go Core 实际 Provider 绑定或多轮场景兼容。
- `pg_isready` 命令不存在。本任务随后从已缓存的 `pgvector/pgvector:pg16` 镜像启动了仅绑定 `127.0.0.1` 的一次性测试容器，并对其中的基库执行现有迁移到 `0036_effective_life`。该容器与生产数据隔离。

## 已执行测试

| 命令 | 结果 | 说明 |
| --- | --- | --- |
| `go -C apps/core-go test -mod=readonly -count=1 ./...` | PASS，命令墙钟约 3.9 秒 | 数据库门禁测试未执行，不可视作集成通过 |
| `pnpm typecheck` | PASS，约 3.3 秒 | browser-client、core-client、web |
| 聚焦 Working Memory/Prompt/Memory/Context 纯逻辑 Go 测试 | PASS，约 0.38 秒 | 仓库既有覆盖 |
| Web conversation delivery 测试 | PASS，7/7，约 0.75 秒 | 仓库既有覆盖 |
| 迁移静态契约 Go 测试 | PASS，约 0.36 秒 | 非 PostgreSQL 实迁 |
| 首次开启数据库门禁的 Core + migrations 测试 | FAIL；Core 约 216.5 秒，migrations PASS 约 31.5 秒 | 六个 Core 测试因测试基库尚未迁移而缺少 `diagnostic_model_runs` / `fluctlights`，并非产品变更引入 |
| 迁移一次性测试基库后重跑上述六项 | PASS，约 0.74 秒 | 证明首次失败属于测试环境准备；全量数据库套件仍待最后重跑 |

## 正式路径现状

- Ordinary、WakeUp、Reflection 已使用 surface-aware Context Projection；初始化及若干一次性任务仍直用 `ComposeTaskMessages`。
- Eino 原生 Agent Loop 中一次无 Tool 的普通对话按代码是 1 次物理模型请求；每轮 Tool 后追加一次请求。该数量尚未用实际模型运行测量。诊断还会另写一条汇总行，不能直接按行数计算物理调用。
- 工作记忆按 Active、runtime、recent、durable retrieval、summary 顺序选取，按来源引用去重；目前缺少 Episode/Resident 独立可追溯产物。
- 当前态的外观/衣柜由 Effective Life 承载，但总投影结算只检查部分领域版本；初始化还存在 Foundation 文本被其他路径重用的风险。

## 实际请求与资源指标

本次实施前未运行 API/Worker，且 Go Core Provider 尚未绑定当前本地模型。因此正式入口的首次请求文本、Tool 回执、实际 token、延迟、后台任务次数、数据迁移前数量均为 **未测量**，不能以旧报告或测试 fixture 代替。一次性 PostgreSQL 已可用于后续迁移/集成验证；真实 Provider 链路仍需另行验证或标记 BLOCKED。

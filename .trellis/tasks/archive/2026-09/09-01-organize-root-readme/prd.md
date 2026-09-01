# 整理根目录 README

## Goal

将根目录 README 改写为中文项目说明，让新成员能够快速理解 Fluctlight 的产品定位、整体架构、当前能力、目录职责、运行方式与后续演进方向。

## Background and confirmed facts

- 当前运行主线是 Go Core、Go BFF、Vue Web 与 Docker Compose；旧的 Python 目录不在当前 Compose 默认运行链路中。
- Go Core 是领域事实和业务写入的唯一入口，负责人格实例、对话、认知、记忆、自治、媒体、诊断和工作流编排。
- Go BFF 是唯一浏览器公网边界，负责认证会话、CSRF/Origin、DTO 转换、错误收敛、NDJSON 流式转译和媒体代理。
- Web 是 Vue 3/Vite/Pinia 静态应用；Browser/Core Client 是由 OpenAPI 生成的契约客户端。
- Compose 拓扑包含 PostgreSQL/pgvector、Redis Streams、MinIO、Temporal、Go Core、Go Worker、Go BFF 和 Nginx Web，并通过 migrate/cutover/health gate 管理启动顺序。

## Requirements

1. 根 README 全文使用中文，保留命令、环境变量、路径、服务名和必要的专有名词原文。
2. README 必须说明项目定位：这是支持持续身份、记忆、关系、生活上下文和受约束自治的 Fluctlight 系统，而不是泛用聊天 UI。
3. README 必须用一段架构图或等价的结构化说明，表达 Web → BFF → Core → PostgreSQL/Redis/Temporal/MinIO/Provider 的边界与数据流，并明确 Core 是唯一领域写入者、BFF 是唯一浏览器边界。
4. README 必须说明当前可见能力，至少覆盖所有者认证、Fluctlight 创建与治理、直接对话、动态、记忆/关系/自治管理、媒体、设置和诊断/工作流控制。
5. README 必须提供主要目录职责表，覆盖 `apps/`、`packages/` 和 `infra/` 下的当前运行组件、契约层、验收与备份工具。
6. README 必须说明本地开发检查、Compose 部署入口、私有环境文件、URL-safe 凭据、首次 Owner 初始化及推荐的整栈 smoke 验证命令。
7. README 必须将后续方向写成基于现有代码扩展点的路线，包括能力插件扩展、向量检索增强、媒体类型扩展、工作流/版本演进和契约同步；不得把尚未接入 Web 的 API 描述为已完成的 UI 功能。
8. 不修改业务代码、API 契约、部署配置或子目录 README；仅整理根 README 与本任务规划记录。

## Acceptance Criteria

- [x] `README.md` 为中文且结构清晰，读者无需阅读源码即可理解定位、架构、能力、目录和方向。
- [x] 架构描述与 `infra/compose/fluctlight.compose.yml`、Go Core/BFF/Web 的实际边界一致，没有把旧 Python 运行时或生成客户端误写成当前生产服务。
- [x] 能力清单区分“当前 Web 已提供”与“底层已具备但尚未在页面接入”的能力。
- [x] README 中的命令和环境变量与现有脚本/配置一致；至少能定位到 `pnpm` 检查、Compose smoke、初始化 token 和部署配置示例。
- [x] 文档变更不引入 Markdown 结构错误、失效的本地路径或与现有领域术语冲突的旧称。

## Notes

- 这是轻量文档任务，保留 PRD 即可，不单独创建 `design.md` 或 `implement.md`。
- 规划依据：`apps/core-go/internal/core`、`apps/core-go/internal/httpapi`、`apps/core-go/internal/workflow`、`apps/gateway-go/internal/bff`、`apps/web/src`、`packages/*`、`infra/compose/fluctlight.compose.yml` 及现有根 README。

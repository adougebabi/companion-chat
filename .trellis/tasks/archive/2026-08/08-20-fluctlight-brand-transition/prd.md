# Fluctlight 品牌层改名（集成审计）

## Goal

确认父任务 `08-20-fluctlight-architecture-performance-modernization` 已将活跃 UI、README 和浏览器 title 统一为 `摇光（Fluctlight）` 及“摇光实例”，并验证旧技术标识的迁移兼容边界。本子任务不重复修改同一批文件。

## Confirmed Parent Delivery

- Parent work commit: `925795c feat: rename active companion product to fluctlight`.
- Changed surface: `README.md`, `src/index.html`, `src/companion-main.js` and the frontend presentation naming guideline.
- Parent verification: 58 tests passed, active-client syntax checks passed, Express title/static smoke passed, and legacy compatibility identifiers remained present.

## Requirements

- 更新活跃 UI 的品牌按钮、侧栏 wordmark、浏览器 title、无障碍标签和 README 主标题/副标题。
- 用户面向文案不得预设统一使用“陪伴者”；必须先落实父任务的术语层级和领域 glossary。
- 用户面向文案需要能表达“一个具备持续自我的 AI 人格实例”与“用户和它之间的关系”是两件不同的事；Fluctlight 是描述这种目标概念的词。
- 单个 AI 实例应使用它自己的名字与“人格”等实例层词汇，不把 Fluctlight 当作它的本体名称。
- `persona` / `companion` 不再作为新的 canonical domain terms；当前代码中的旧技术标识是否全量迁移，另立兼容迁移任务决定。
- 不改 `/api/companion`、`companion_*` 表、`companion.sqlite`、`COMPANION_*` 环境变量、Docker volume、localStorage key 和 legacy 资源名。
- 不在未引用的 `src/main.js` / `src/style.css` 上扩大改动范围。

## Acceptance Criteria

- [x] 活跃 UI、README、document.title 和 accessible brand labels 使用经过确认的 Fluctlight 术语体系。
- [x] 在没有批准全量迁移前，旧 localStorage 选择、API 请求、Docker/SQLite 标识和测试契约仍可用。
- [x] 全仓库检查能区分展示层 Fluctlight 和内部 companion，不产生半改名状态。
- [x] 本子任务不再重复修改 parent 已交付的展示层文件；后续 `web/` -> `dist/` 前端迁移另行验证最终入口。

## Dependencies And Out Of Scope

- 已由 parent integration review 覆盖当前 `src/` 活跃入口；未来 `web/` -> `dist/` 入口由 frontend child 在迁移验收时复核相同术语边界。
- 不做商标/域名注册，不做 API/数据库 major rename，不修改 legacy UI 的设计系统。

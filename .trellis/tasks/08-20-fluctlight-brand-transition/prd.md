# Fluctlight 品牌层改名

## Goal

在领域术语关系确定后，将活跃 UI、README 和浏览器 title 统一为 `摇光（Fluctlight）` 这一产品/领域概念及“摇光实例”的实例词，同时保留旧技术标识的迁移兼容边界。

## Requirements

- 更新活跃 UI 的品牌按钮、侧栏 wordmark、浏览器 title、无障碍标签和 README 主标题/副标题。
- 用户面向文案不得预设统一使用“陪伴者”；必须先落实父任务的术语层级和领域 glossary。
- 用户面向文案需要能表达“一个具备持续自我的 AI 人格实例”与“用户和它之间的关系”是两件不同的事；Fluctlight 是描述这种目标概念的词。
- 单个 AI 实例应使用它自己的名字与“人格”等实例层词汇，不把 Fluctlight 当作它的本体名称。
- `persona` / `companion` 不再作为新的 canonical domain terms；当前代码中的旧技术标识是否全量迁移，另立兼容迁移任务决定。
- 不改 `/api/companion`、`companion_*` 表、`companion.sqlite`、`COMPANION_*` 环境变量、Docker volume、localStorage key 和 legacy 资源名。
- 不在未引用的 `src/main.js` / `src/style.css` 上扩大改动范围。

## Acceptance Criteria

- [ ] 活跃 UI、README、document.title 和 accessible brand labels 使用经过确认的 Fluctlight 术语体系。
- [ ] 在没有批准全量迁移前，旧 localStorage 选择、API 请求、Docker/SQLite 标识和测试契约仍可用。
- [ ] 全仓库检查能区分展示层 Fluctlight 和内部 companion，不产生半改名状态。

## Dependencies And Out Of Scope

- 依赖 frontend child 确认最终渲染入口和父任务 integration review。
- 不做商标/域名注册，不做 API/数据库 major rename，不修改 legacy UI 的设计系统。

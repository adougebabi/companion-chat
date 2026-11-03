# Fluctlight / AEL 项目定义

AEL 只管理本仓库的工程任务、约束、验证和交付记录；业务人格与运行时概念由 [CONTEXT.md](../CONTEXT.md) 定义。

工程规则统一入口：[project-entry](knowledge/spec/project-entry.md)。项目使用 Go Core 与 React 前端；已有具体合同和已确认用户规则不由初始化重写。原始 .trellis 仅保留历史与来源。

检查候选来自现有 package.json 与 Go 模块。按每项任务的实际范围配置验证，不在初始化时自动运行安装、构建、hooks、模型、数据库迁移或发布。目标是验收通过改动时不强制生产发布；目标为合并/发布时需准确目标与实际回执。

当前模型执行器尚未自动启用；Attached CLI 可用。未填写的发布环境、远程地址和业务验收结果不作推定。
